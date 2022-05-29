package main

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	Block int = 0 // *, blocks all
	Allow     = 1 // *, allows all
)

type Container struct {
	id       string
	template Template             // template data (image, tags, etc)
	tag      any                  // will be nil (default template tag) or a string
	envs     map[string]string    // forced envs
	localId  any                  // vm id (docker container id)
	machine  *Machine             // TODO check if circular references break the code
	ports    map[int]FirewallRule // port + protocol
	storage  *Storage             // the storage path for this container. it can be local or remote, it could also be null if not hosted yet
	removed  any                  // will be Time if the container was removed
	ip       any
}

func (c Container) PullImage() (err error) {
	// TODO safe logging, + ugly af
	c.machine.Log("pull image with name '"+c.getFullImage()+"'", Debug)
	reader, err := c.machine.cli.ImagePull(c.machine.ctx, c.getFullImage(), types.ImagePullOptions{})
	if err != nil {
		return err
	}
	err = reader.Close()
	c.machine.Log("pulled image '"+c.getFullImage()+"'", Debug)
	return err
}

func (c Container) getParsedEnvs() (parsedEnvs []string) {
	for env, val := range c.envs {
		parsedEnvs = append(parsedEnvs, env+"="+val)
	}
	return parsedEnvs
}

func (c Container) getFullImage() (image string) {
	imageSuffix := ""
	if c.tag != nil {
		imageSuffix = ":" + fmt.Sprintf("%v", c.tag)
	}
	return c.template.image + imageSuffix
}

func (c Container) getPortBindings() (bindings nat.PortMap, portSet nat.PortSet, err error) {
	bindings = make(nat.PortMap)
	portSet = make(nat.PortSet)
	for port, rule := range c.ports {

		var neededProtocols []string
		if rule.protocol == BOTH {
			neededProtocols = append(neededProtocols, TCP, UDP)
		} else {
			neededProtocols = append(neededProtocols, rule.protocol)
		}

		for _, protocol := range neededProtocols {

			newPort, err := nat.NewPort(protocol, strconv.Itoa(port))
			if err != nil {
				return bindings, portSet, err
			}

			portSet[newPort] = struct{}{}

			var finalIP string
			if c.ip == nil {
				finalIP = "0.0.0.0"
			} else {
				finalIP = fmt.Sprintf("%v", c.ip)
			}

			bindings[newPort] = []nat.PortBinding{
				{
					HostIP:   finalIP,
					HostPort: strconv.Itoa(port),
				},
			}
		}

	}

	return bindings, portSet, nil
}

func (c Container) IsExpired() (expired bool) {
	t := c.removed.(time.Time)
	return c.template.expiry >= 0 && time.Now().Unix()-t.Unix() > c.template.expiry
}

func (c Container) getMountPath() (path string, err error) {
	if c.storage == nil {
		return "", errors.New("this container doesn't have an attached storage method")
	}
	return filepath.FromSlash(c.storage.path + "/" + c.id), nil
}

func (c *Container) Create() (err error) {
	// 0. check if the image already exists
	if c.localId != nil {
		return errors.New("this container already exists")
	}
	// 1. look for a suitable hardware resources
	for _, storage := range c.machine.storage {
		available := storage.GetAvailableSpace()
		if storage.size == 0 || available >= c.template.size {
			c.storage = &storage
			break
		}
	}
	if c.storage == nil {
		return errors.New("not enough space")
	}
	resources := container.Resources{}
	if c.template.memory > 0 && c.machine.hardware.GetAvailableMemory() < uint64(c.template.memory) {
		return errors.New("not enough memory")
	} else if c.template.memory > 0 {
		resources.Memory = int64(c.template.memory)
		resources.MemorySwap = resources.Memory
	}
	if c.template.cores > 0 && c.machine.hardware.GetAvailableCores() < c.template.cores {
		return errors.New("not enough cpu cores")
	} else if c.template.cores > 0 {
		resources.CPUPeriod = 100000
		resources.CPUQuota = int64(100000 * c.template.cores)
	}
	// 2. pull image
	err = c.PullImage()
	if err != nil {
		return err
	}
	// 3. create container
	networkBinds, portSet, err := c.getPortBindings()
	if err != nil {
		return err
	}
	mountPath, err := c.getMountPath()
	if err != nil {
		return err
	}
	hostConfig := container.HostConfig{
		PortBindings: networkBinds,
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: mountPath,
				Target: c.template.mount,
			},
		},
		Resources: resources,
	}
	if c.template.size > 0 {
		hostConfig.StorageOpt = map[string]string{ // https://stackoverflow.com/questions/57248180/docker-per-container-disk-quota-on-bind-mounted-volumes
			"size": fmt.Sprintf("%dB", c.template.size),
		}
	}
	create, err := c.machine.cli.ContainerCreate(c.machine.ctx, &container.Config{
		Image:        c.getFullImage(),
		Env:          c.getParsedEnvs(),
		Tty:          true,
		ExposedPorts: portSet,
	}, &hostConfig, nil, nil, c.machine.getFullPrefix()+c.id)
	if err != nil {
		return err
	}
	// 3. container created, attach container id to the container data
	c.localId = create.ID
	return nil
}

func (c Container) GetLocalId() (id string, err error) {
	if c.localId == nil {
		return "", errors.New("this container isn't locally present")
	}
	return fmt.Sprintf("%v", c.localId), err
}

func (c *Container) Delete(completeDelete bool) (err error) {
	// will delete the container instantly; the data should be wiped in 48h or so
	id, err := c.GetLocalId()
	if err != nil {
		return err
	}
	err = c.machine.cli.ContainerRemove(c.machine.ctx, id, types.ContainerRemoveOptions{
		Force: true,
	})
	// TODO wipe queue
	// remove local id
	c.localId = nil
	c.removed = time.Now()
	if completeDelete {
		err := c.ClearStorage()
		if err != nil {
			return err
		}
		c.RemoveFromList()
	}
	return err
}

func (c Container) getIndex() (i int) {
	for i, cont := range c.machine.containers {
		if cont.id == c.id {
			return i
		}
	}
	return -1
}

func (c *Container) RemoveFromList() {
	s := c.getIndex()
	if s >= 0 {
		c.machine.containers = append(c.machine.containers[:s], c.machine.containers[s+1:]...)
	}
}

func (c Container) GetDeleted() (deleted any) {
	return c.removed
}

func (c *Container) ClearStorage() (err error) {
	c.machine.Log("clearing storage for container #"+c.id, Debug)
	path, err := c.getMountPath()
	if err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func (c Container) Start() (err error) {
	id, err := c.GetLocalId()
	if err != nil {
		return err
	}
	err = c.machine.cli.ContainerStart(c.machine.ctx, id, types.ContainerStartOptions{})
	return err
}

func (c Container) Restart() (err error) {
	id, err := c.GetLocalId()
	if err != nil {
		return err
	}
	err = c.machine.cli.ContainerRestart(c.machine.ctx, id, nil)
	return err
}

func (c Container) Stop() (err error) {
	id, err := c.GetLocalId()
	if err != nil {
		return err
	}
	err = c.machine.cli.ContainerStop(c.machine.ctx, id, nil)
	return err
}

func (c Container) Kill() (err error) {
	id, err := c.GetLocalId()
	if err != nil {
		return err
	}
	err = c.machine.cli.ContainerKill(c.machine.ctx, id, "SIGKILL")
	return err
}

func (c Container) Pause() (err error) {
	id, err := c.GetLocalId()
	if err != nil {
		return err
	}
	err = c.machine.cli.ContainerPause(c.machine.ctx, id)
	return err
}

func (c Container) Resume() (err error) {
	id, err := c.GetLocalId()
	if err != nil {
		return err
	}
	err = c.machine.cli.ContainerUnpause(c.machine.ctx, id)
	return err
}
