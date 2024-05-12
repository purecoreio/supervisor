package container

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"path"
	"supervisor/machine/container/stream"
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
	stream   map[string]stream.Stream
}

func (i Container) GetStream(rid string, conn *websocket.Conn) stream.Stream {
	if i.stream == nil {
		i.stream = make(map[string]stream.Stream)
	}
	activeStream, ok := i.stream[rid]
	if !ok {
		i.stream[rid] = stream.Stream{
			Path: i.Path,
			Id:   i.Id,
			Cid:  i.name(),
			Rid:  rid,
			Conn: conn,
		}
		activeStream = i.stream[rid]
	}
	return activeStream
}

func (i Container) logger() (entry *log.Entry) {
	return log.WithFields(log.Fields{
		"container": i.Id,
	})
}

func (i Container) name() (name string) {
	return "sb-" + i.Id
}

func (i Container) Start(cli *client.Client, ctx context.Context) (err error) {
	i.logger().Info("starting container")
	_, err = cli.ImagePull(ctx, "docker.io/"+i.Template.Image.Uri, image.PullOptions{})
	if err != nil {
		i.logger().Error("unable to pull image: " + err.Error())
		return err
	}
	config := &container.Config{
		Image: "itzg/minecraft-server",
		Env:   []string{"EULA=true"},
	}
	i.logger().Info(path.Join(i.Path, "data"))
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Target: "/data",
				Source: path.Join(i.Path, "data"),
			},
		},
	}
	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, i.name())
	if err != nil {
		i.logger().Error("unable to create container: " + err.Error())
		return err
	}
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		i.logger().Error("unable to start container: " + err.Error())
		return err
	}
	i.logger().Info("started container")
	return err
}

func (i Container) Username() (username string) {
	return "sb-" + i.Id
}
