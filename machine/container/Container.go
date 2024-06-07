package container

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
	"github.com/thanhpk/randstr"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"supervisor/machine/container/ip"
	"supervisor/machine/container/listener"
	"supervisor/machine/container/listener/activity"
	"supervisor/machine/container/listener/event"
)

type Container struct {
	Id         string            `json:"id"`
	Template   HostingTemplate   `json:"template"`
	Ports      []ip.Port         `json:"ports"`
	Envs       map[string]string `json:"envs"`
	Path       string            `json:"path"`
	Memory     int               `json:"memory"`
	Storage    *int              `json:"storage"`
	Repository *Repository       `json:"repository,omitempty"`
	Branch     *string           `json:"branch,omitempty"`
	Handler    *listener.Handler
}

var (
	group     = "serverbench"
	directory = "/etc/serverbench/containers/"
)

func (c *Container) Init(out *chan event.Entry, cli *client.Client) (err error) {
	if c.Handler != nil {
		return errors.New("container already initialized")
	}
	handler := listener.Handler{
		Status:        make([]listener.Subscriber, 0),
		Logs:          make([]listener.Subscriber, 0),
		Progress:      make([]listener.Subscriber, 0),
		Load:          make([]listener.Subscriber, 0),
		ContainerId:   c.Id,
		ContainerName: c.Username(),
		Client:        cli,
		ProgressCache: make(map[string]event.ProgressUpdate),
	}
	err = handler.Forward(out)
	if err == nil {
		c.Handler = &handler
	}
	return err
}

func (c *Container) isGitRepository() (isRepo bool, err error) {
	dataPath := path.Join(c.Path, "data")
	gitDir := path.Join(dataPath, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

func (c *Container) getState(cli *client.Client) (state *types.ContainerState, err error) {
	inspect, err := cli.ContainerInspect(context.Background(), c.Username())
	if err != nil {
		return state, err
	}
	if inspect.State == nil {
		err = errors.New("no container state")
		return state, err
	}
	return inspect.State, err
}

func (c *Container) Stop(cli *client.Client) (err error) {
	state, err := c.getState(cli)
	if err != nil {
		return err
	}
	if state.Paused {
		err = errors.New("container is frozen")
		return err
	}
	err = cli.ContainerStop(context.Background(), c.Username(), container.StopOptions{})
	return err
}

func (c *Container) Pull(cli *client.Client, token *string, headSha *string) (err error) {
	c.logger().Info("pulling repository")
	if c.Branch == nil {
		c.logger().Error("missing repository branch")
		err = errors.New("pull needs a branch")
		return err
	}
	if c.Repository == nil {
		c.logger().Error("missing repository uri")
		err = errors.New("pull needs a repository")
		return err
	}
	if token == nil {
		c.logger().Error("missing repository token")
		err = errors.New("pull needs a token")
		return err
	}
	// check username and token
	err = c.Repository.CheckToken(*token)
	if err != nil {
		c.logger().Error("invalid repository token")
		return err
	}
	// check if git repo is initialized
	isRepo, err := c.isGitRepository()
	if err != nil {
		c.logger().Error("error while checking if project is within a git repository: ", err)
		return err
	}
	// check state is valid for pull
	state, err := c.getState(cli)
	if err != nil {
		c.logger().Error("error while checking if state is valid for pull: ", err)
		return err
	}
	shouldRestart := false
	if state.Paused {
		c.logger().Error("unable tu pull while frozen")
		err = errors.New("unable to perform pull while the container is frozen")
		return err
	} else if state.Running {
		c.logger().Info("stopping container in preparation for pull - container will be restarted when finished")
		shouldRestart = true
		err = c.Stop(cli)
		if err != nil {
			return err
		}
	}
	gitUrl := "https://x-access-token:" + *token + "@github.com/" + c.Repository.Uri
	isUpdated := false
	dataPath := path.Join(c.Path, "data")
	// if the repo is not initialized, we will first pull aside the data, perform a clone, and move data back
	if !isRepo {
		c.logger().Info("the container is not on a github repository, initializing")
		temporaryId, err := c.pullAside()
		if err != nil {
			return err
		}
		c.logger().Info("initializing container")
		cloneActivity := activity.Activity{
			Command:          exec.Command("git", "-C", dataPath, "clone", "--progress", "-b", *c.Branch, gitUrl, "."),
			Description:      "clone " + c.Repository.Uri,
			ProgressRegex:    activity.GenericPercentRegex,
			ProgressIndex:    1,
			DescriptionRegex: activity.GenericDescriptionColonRegex,
			DescriptionIndex: 1,
			HeadSha:          headSha,
			Type:             "git",
		}
		err = cloneActivity.Exec(c.Handler)
		if err != nil {
			c.logger().Error("error while initializing: ", err)
			_ = c.bringTogether(temporaryId)
			return err
		}
		err = c.bringTogether(temporaryId)
		if err != nil {
			return err
		}
		isUpdated = true
	}
	// clean repo
	c.logger().Info("whitelisting repo")
	err = exec.Command("git", "config", "--global", "--add", "safe.directory", dataPath).Run()
	if err != nil {
		c.logger().Error("error while whitelisting repo")
		return err
	}
	c.logger().Info("resetting repo")
	err = exec.Command("git", "-C", dataPath, "reset", "--hard").Run()
	if err != nil {
		c.logger().Error("error resetting repo: ", err)
		return err
	}
	c.logger().Info("cleaning up repo")
	err = exec.Command("git", "-C", dataPath, "clean", "-dff").Run()
	if err != nil {
		c.logger().Error("error cleaning up repo: ", err)
		return err
	}
	if !isUpdated {
		// update remote url (token)
		c.logger().Info("updating remote")
		err = exec.Command("git", "-C", dataPath, "remote", "set-url", "origin", gitUrl).Run()
		if err != nil {
			c.logger().Error("error while updating remote")
			return err
		}
		// ensure correct branch
		c.logger().Info("checking out branch")
		err = exec.Command("git", "-C", dataPath, "checkout", *c.Branch).Run()
		if err != nil {
			c.logger().Error("error while checking out branch: ", err)
			return err
		}
		// pull changes
		c.logger().Info("pulling changes")
		pullActivity := activity.Activity{
			Command:          exec.Command("git", "-C", dataPath, "pull", "--progress", "--rebase"),
			Description:      "pulling " + c.Repository.Uri,
			ProgressRegex:    activity.GenericPercentRegex,
			ProgressIndex:    1,
			DescriptionRegex: activity.GenericDescriptionColonRegex,
			DescriptionIndex: 1,
			HeadSha:          headSha,
			Type:             "git",
		}
		err = pullActivity.Exec(c.Handler)
		if err != nil {
			c.logger().Info("error while pulling changes: ", err)
			return err
		}
	}
	if shouldRestart {
		c.logger().Info("restarting the container to match the initial state before pull")
		err = c.Start(cli, nil, nil)
	}
	return err
}

func (c *Container) pullAside() (temporaryId string, err error) {
	c.logger().Info("pulling aside")
	temporaryId = randstr.Hex(8)
	targetPath := path.Join(c.Path, "data-"+temporaryId)
	err = os.MkdirAll(targetPath, os.ModePerm)
	if err != nil {
		return "", err
	}
	originPath := path.Join(c.Path, "data")
	r, err := exec.Command("rsync", "-a", "--remove-source-files", c.appendSlash(originPath), targetPath).Output()
	if err != nil {
		c.logger().Error("error while pulling aside, trying to bring together: ", string(r), ", ", err)
		_ = c.bringTogether(temporaryId)
		return "", err
	}
	return temporaryId, err
}

func (c *Container) bringTogether(temporaryId string) (err error) {
	c.logger().Info("bringing together aside")
	temporaryDirectory := path.Join(c.Path, "data-"+temporaryId)
	originPath := path.Join(c.Path, "data")
	r, err := exec.Command("rsync", "-a", "--remove-source-files", "--ignore-existing", c.appendSlash(temporaryDirectory), originPath).Output()
	if err != nil {
		c.logger().Error("error while bringing together: ", string(r), ", ", err)
		return err
	}
	// cleanup
	err = os.Remove(temporaryDirectory)
	if err != nil {
		c.logger().Error("error while cleaning after bringing together")
	}
	return err
}

func (c *Container) appendSlash(str string) string {
	if len(str) == 0 || str[len(str)-1] != os.PathSeparator {
		return str + string(os.PathSeparator)
	}
	return str
}

func (c *Container) Transfer(out bool, externalAddress string, externalPort int, externalDirectory string, externalUser string, mirror bool, externalPassword *string, headSha *string) (err error) {
	c.logger().Info("starting transfer")
	var commands []string
	sshArgs := []string{"ssh", "-o", "StrictHostKeyChecking=no", "-p", strconv.Itoa(externalPort)}
	if externalPassword != nil {
		c.logger().Info("transfer uses password")
		commands = append(commands, "sshpass", "-p", strconv.Quote(*externalPassword))
	} else {
		c.logger().Info("transfer uses ssh keys")
		sshArgs = append(sshArgs, "-i", c.getPrivateKeyFile())
	}
	commands = append(commands, "rsync", "-e", strconv.Quote(strings.Join(sshArgs, " ")), "-az", "--no-inc-recursive", "--info=progress2", "--update")
	if mirror {
		c.logger().Info("transfer mirrors data")
		commands = append(commands, "--delete")
	}
	externalSnippet := externalUser + "@" + externalAddress + ":" + externalDirectory
	localSnippet := path.Join(c.Path, "data")
	var from string
	var to string
	var description string
	if out {
		c.logger().Info("transfer uploads data")
		from = localSnippet
		to = externalSnippet
		description = "transfering to " + externalSnippet
	} else {
		c.logger().Info("transfer downloads data")
		from = externalSnippet
		to = localSnippet
		description = "transfering from " + externalSnippet
	}
	commands = append(commands, strconv.Quote(c.appendSlash(from)), strconv.Quote(to))
	transferActivity := activity.Activity{
		Command:          exec.Command("sh", "-c", strings.Join(commands, " ")),
		Description:      description,
		DescriptionIndex: 0,
		DescriptionRegex: nil,
		ProgressIndex:    1,
		ProgressRegex:    activity.GenericPercentRegex,
		HeadSha:          headSha,
		Type:             "transfer",
	}
	err = transferActivity.Exec(c.Handler)
	if err != nil {
		c.logger().Info("error while transferring: ", err)
		return err
	}
	return nil
}

func (c *Container) Host(cli *client.Client, containers map[string]Container, token *string, headSha *string) (err error) {
	c.logger().Info("hosting")
	exists, err := c.userExists()
	if err != nil {
		return err
	}
	if !exists {
		_, err = c.createUser()
		if err != nil {
			return err
		}
	}
	containers[c.Id] = *c

	// container should mount volume onto settings.path/data
	go func() {
		_ = c.Start(cli, token, headSha)
	}()
	c.logger().Info("hosted " + c.Id)
	return nil
}

func (c *Container) Unhost(cli *client.Client, containers map[string]Container) (err error) {
	c.logger().Info("unhosting " + c.Id)
	_ = c.Kill(cli)
	err = c.removeUser()
	if err != nil {
		return err
	}
	delete(containers, c.Id)
	c.logger().Info("unhosted " + c.Id)
	return nil
}

func (c *Container) Kill(cli *client.Client) (err error) {
	c.logger().Info("killing")
	err = cli.ContainerRemove(context.Background(), c.Username(), container.RemoveOptions{
		Force: true,
	})
	if err != nil {
		c.logger().Error("unable to kill: ", err)
	} else {
		c.logger().Info("killed")
	}
	return err
}

func (c *Container) logger() (entry *log.Entry) {
	return log.WithFields(log.Fields{
		"container": c.Id,
	})
}

func (c *Container) containerExists(cli *client.Client) (exists bool, err error) {
	list, err := cli.ContainerList(context.Background(), container.ListOptions{
		All: true,
	})
	if err != nil {
		return false, err
	}
	for _, ctr := range list {
		if ctr.Names[0] == "/"+c.Username() {
			return true, nil
		}
	}
	return false, nil
}

func (c *Container) Start(cli *client.Client, token *string, headSha *string) (err error) {
	ctx := context.Background()
	exists, err := c.containerExists(cli)
	if err != nil {
		return err
	}
	if exists {
		err = c.Stop(cli)
		if err != nil {
			return err
		}
		err = cli.ContainerRemove(context.Background(), c.Username(), container.RemoveOptions{
			Force: true,
		})
		if err != nil {
			return err
		}
	}
	c.logger().Info("starting container")
	_, err = cli.ImagePull(ctx, "docker.io/"+c.Template.Image.Uri, image.PullOptions{})
	if err != nil {
		c.logger().Error("unable to pull image: " + err.Error())
		return err
	}
	parsedEnvs := make([]string, 0)
	for k, v := range c.Envs {
		parsedEnvs = append(parsedEnvs, fmt.Sprintf("%s=%s", k, v))
	}
	config := &container.Config{
		Image:     c.Template.Image.Uri,
		Env:       parsedEnvs,
		Tty:       true,
		OpenStdin: true,
	}
	portBindings := nat.PortMap{}
	for _, port := range c.Ports {
		protos := []string{"tcp", "udp"}
		for _, proto := range protos {
			dockerPort, err := nat.NewPort(proto, strconv.Itoa(port.Port))
			if err != nil {
				return err
			}
			portBindings[dockerPort] = []nat.PortBinding{{
				HostIP:   port.Ip.Ip,
				HostPort: strconv.Itoa(port.Port),
			}}
		}
	}
	c.logger().Info(path.Join(c.Path, "data"))
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Target: c.Template.Image.DefaultMount,
				Source: path.Join(c.Path, "data"),
			},
		},
		PortBindings: portBindings,
	}
	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, c.Username())
	if err != nil {
		c.logger().Error("unable to create container: " + err.Error())
		return err
	}
	if token != nil {
		err = c.Pull(cli, token, headSha)
		if err != nil {
			return err
		}
	}
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		c.logger().Error("unable to start container: " + err.Error())
		return err
	}
	c.logger().Info("started container")
	return err
}

func (c *Container) Username() (username string) {
	return "sb-" + c.Id
}
