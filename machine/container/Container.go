package container

import (
	"context"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"github.com/thanhpk/randstr"
	"os"
	"os/exec"
	"path"
	"strings"
	"supervisor/machine/container/listener"
	"supervisor/machine/container/listener/event"
)

type Container struct {
	Id       string            `json:"id"`
	Template HostingTemplate   `json:"template"`
	Ports    map[int]int       `json:"ports"`
	Envs     map[string]string `json:"envs"`
	Path     string            `json:"path"`
	Ip       Ip                `json:"ip"`
	Memory   int               `json:"memory"`
	Storage  *int              `json:"storage"`
	Handler  *listener.Handler
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
	}
	err = handler.Forward(out)
	if err == nil {
		c.Handler = &handler
	}
	return err
}

func (c *Container) isGitRepository() (isRepo bool, err error) {
	dataPath := path.Join(c.Path, "data")
	cmd := exec.Command("git", "-C", dataPath, "rev-parse", "--is-inside-work-tree")
	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			return false, nil
		}
		return false, err
	}

	return strings.TrimSpace(string(output)) == "true", nil
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

func (c *Container) Pull(cli *client.Client, repositoryCredentials *RepositoryCredentials) (err error) {
	// check username and token
	err = repositoryCredentials.CheckToken()
	if err != nil {
		return err
	}
	err = repositoryCredentials.CheckUsername()
	if err != nil {
		return err
	}
	// check if git repo is initialized
	isRepo, err := c.isGitRepository()
	if err != nil {
		return err
	}
	// check state is valid for pull
	state, err := c.getState(cli)
	if err != nil {
		return err
	}
	shouldRestart := false
	if state.Paused {
		err = errors.New("unable to perform pull while the container is frozen")
		return err
	} else if state.Running {
		shouldRestart = true
		err = c.Stop(cli)
		if err != nil {
			return err
		}
	}
	gitUrl := "https://" + repositoryCredentials.Username + ":" + repositoryCredentials.Token + "@" + repositoryCredentials.Repository.Uri
	isUpdated := false
	dataPath := path.Join(c.Path, "data")
	// if the repo is not initialized, we will first pull aside the data, perform a clone, and move data back
	if !isRepo {
		temporaryId, err := c.pullAside()
		if err != nil {
			return err
		}
		err = exec.Command("git", "-C", dataPath, "clone", "-b", repositoryCredentials.Repository.Branch, gitUrl).Run()
		if err != nil {
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
	err = exec.Command("git", "-C", dataPath, "reset", "--hard").Run()
	if err != nil {
		return err
	}
	err = exec.Command("git", "-C", dataPath, "clean", "-dff").Run()
	if err != nil {
		return err
	}
	if !isUpdated {
		// update remote url (token)
		err = exec.Command("git", "-C", dataPath, "remote", "set-url", "origin", gitUrl).Run()
		if err != nil {
			return err
		}
		// ensure correct branch
		err = exec.Command("git", "-C", dataPath, "checkout", repositoryCredentials.Repository.Branch).Run()
		if err != nil {
			return err
		}
		// pull changes
		err = exec.Command("git", "-C", dataPath, "pull", "--rebase").Run()
		if err != nil {
			return err
		}
	}
	if shouldRestart {
		err = c.Start(cli, nil)
	}
	return err
}

func (c *Container) pullAside() (temporaryId string, err error) {
	temporaryId = randstr.Hex(8)
	targetPath := path.Join(c.Path, "data-"+temporaryId)
	err = os.MkdirAll(targetPath, os.ModePerm)
	if err != nil {
		return "", err
	}
	originPath := path.Join(c.Path, "data")
	originData := path.Join(originPath, "*")
	err = exec.Command("mv", originData, targetPath).Run()
	if err != nil {
		_ = c.bringTogether(temporaryId)
		return "", err
	}
	return temporaryId, err
}

func (c *Container) bringTogether(temporaryId string) (err error) {
	temporaryDirectory := path.Join(c.Path, "data-"+temporaryId)
	temporaryData := path.Join(temporaryDirectory, "*")
	originPath := path.Join(c.Path, "data")
	err = exec.Command("mv", "-n", temporaryData, originPath).Run()
	if err != nil {
		return err
	}
	// cleanup
	err = os.Remove(temporaryDirectory)
	return err
}

func (c *Container) Host(cli *client.Client, containers map[string]Container, repositoryCredentials *RepositoryCredentials) (err error) {
	c.logger().Info("hosting")
	_, err = c.createUser()
	if err != nil {
		return err
	}
	containers[c.Id] = *c

	// container should mount volume onto settings.path/data
	go func() {
		_ = c.Start(cli, repositoryCredentials)
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

func (c *Container) Start(cli *client.Client, repositoryCredentials *RepositoryCredentials) (err error) {
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
	config := &container.Config{
		Image:     "itzg/minecraft-server",
		Env:       []string{"EULA=true"},
		Tty:       true,
		OpenStdin: true,
	}
	c.logger().Info(path.Join(c.Path, "data"))
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Target: "/data",
				Source: path.Join(c.Path, "data"),
			},
		},
	}
	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, c.Username())
	if err != nil {
		c.logger().Error("unable to create container: " + err.Error())
		return err
	}
	if repositoryCredentials != nil {
		err = c.Pull(cli, repositoryCredentials)
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
