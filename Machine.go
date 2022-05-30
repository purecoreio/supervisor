package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/pbnjay/memory"
	"github.com/shirou/gopsutil/v3/cpu"
	"runtime"
	"strings"
	"time"
)

const (
	Debug    int = -1
	Critical     = 0
	Info         = 1
)

type Machine struct {
	id         string
	hardware   Hardware
	containers []Container
	storage    []Storage
	ctx        context.Context
	cli        *client.Client
}

func (m *Machine) loadDocker() (err error) {
	m.ctx = context.Background()
	m.cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	return err
}

func (m *Machine) GetContainer(id string) (container *Container, err error) {
	for _, c := range m.containers {
		if c.id == id {
			return &c, nil
		}
	}
	return &Container{}, errors.New("unknown container")
}

func (m *Machine) UpdateHardware() (err error) {
	m.hardware = Hardware{}
	info, _ := cpu.Info()
	m.hardware.CPU = info[0].ModelName
	m.Log("cpu model: "+m.hardware.CPU, Debug)
	counts, err := cpu.Counts(true)
	if err != nil {
		return err
	}
	m.hardware.coreCount = uint(counts)
	m.Log("core count: "+fmt.Sprintf("%v", m.hardware.coreCount), Debug)
	m.hardware.memory = memory.TotalMemory()
	m.Log("installed memory: "+fmt.Sprintf("%vMB", m.hardware.memory/1024/1024), Debug)
	m.hardware.machine = m
	return nil
}

func (m Machine) GetDataFolder() (path string) {
	if runtime.GOOS == "windows" {
		return "%appdata%\\purecore\\machine-supervisor"
	} else {
		return "/etc/purecore/machine-supervisor"
	}
}

func (m *Machine) Setup() (err error) {
	if runtime.GOOS != "windows" {
		err = m.setupSSHD()
		if err != nil {
			return err
		}
	} else {
		m.Log("the account manager isn't available, since this isn't running on a linux-based system", Critical)
	}
	err = m.loadDocker()
	if err != nil {
		return err
	}
	err = m.listContainers()
	if err != nil {
		return err
	}
	err = m.UpdateHardware()
	if err != nil {
		return err
	}
	return err
}

func (m *Machine) listContainers() (err error) {
	list, err := m.cli.ContainerList(m.ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return err
	}
	for _, container := range list {
		for _, name := range container.Names {
			name = strings.TrimPrefix(name, "/")
			if strings.HasPrefix(name, m.getFullPrefix()) {

				id := strings.TrimPrefix(name, m.getFullPrefix())
				foundContainer, err := m.GetContainer(id)

				// attach local id to local container
				foundContainer.localId = container.ID
				foundContainer.machine = m

				// attach storage info to the container
				foundContainer.id = id // just in case the container is orphan
				desktopPrefix := "/run/desktop/mnt/host/"
				hyperPrefix := "/host_mnt/"
				currentPath := container.Mounts[0].Source
				currentPath = strings.TrimPrefix(currentPath, desktopPrefix)
				currentPath = strings.TrimPrefix(currentPath, hyperPrefix)
				currentPath = strings.TrimSuffix(currentPath, "/"+id)

				// special case for windows, drive letter conversion
				if runtime.GOOS == "windows" {
					currentPath = strings.ToUpper(string(currentPath[0])) + ":" + currentPath[1:]
				}

				foundContainer.storage = &Storage{
					path: currentPath,
				}

				if err != nil {
					// this container isn't present on the container list, we may consider it as an orphan container
					m.Log("removing orphan container #"+foundContainer.id+": "+err.Error(), Debug)
					err := foundContainer.Delete(true)
					if err != nil {
						m.Log("error while removing unknown orphan container #"+container.ID+": "+err.Error(), Critical)
					}
				}
				break
			}
		}
	}
	return nil
}

func (m *Machine) Worker() {
	for range time.Tick(1 * time.Second) {
		m.ClearExpired()
	}
}

func (m *Machine) ClearExpired() {
	for _, container := range m.containers {
		deleted := container.GetDeleted()
		if deleted != nil {
			if container.IsExpired() {
				err := container.ClearStorage()
				if err != nil {
					m.Log("error while removing expired data from container #"+container.id+": "+err.Error(), Critical)
				}
				container.RemoveFromList()
			}
		}
	}
}

func (m Machine) GetPrefix() (prefix string) {
	return "purecore"
}

func (m Machine) getFullPrefix() (fullPrefix string) {
	return m.GetPrefix() + "-"
}

func (m Machine) Log(message string, error int) {
	prefix := ""
	switch error {
	case 0:
		prefix = "CRITICAL"
		break
	case 1:
		prefix = "INFO"
		break
	default:
		prefix = "DEBUG"
	}
	fmt.Println("[" + prefix + "] " + message)
	// TODO implement log registry
}
