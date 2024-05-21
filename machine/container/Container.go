package container

import (
	"context"
	"errors"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"path"
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

func (c *Container) Host(cli *client.Client, containers map[string]Container) (err error) {
	c.logger().Info("hosting")
	_, err = c.createUser()
	if err != nil {
		return err
	}
	containers[c.Id] = *c

	// container should mount volume onto settings.path/data
	go func() {
		_ = c.Start(cli, context.Background())
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

func (c *Container) Start(cli *client.Client, ctx context.Context) (err error) {
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
