package main

import (
	"fmt"
	"math/rand"
	"time"
)

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func main() {

	machine := Machine{
		storage: []Storage{
			{
				size: 0,
				path: "C:\\Users\\happy\\Documents\\purecore-containers",
			},
		},
		containers: []Container{
			{
				id: randomString(16),
				template: Template{
					image:  "itzg/minecraft-server",
					size:   0,
					mount:  "/data",
					expiry: 0,
					memory: 1024 * 1024 * 1024 * 3,
					cores:  0.5,
				},
				tag: nil,
				envs: map[string]string{
					"EULA": "TRUE",
					"TYPE": "PAPER",
				},
				ports: map[int]FirewallRule{
					25565: {
						protocol: TCP,
					},
				},
			},
		},
	}
	err := machine.Setup()
	machine.containers[0].machine = &machine
	machine.storage[0].machine = &machine
	go machine.Worker()
	if err != nil {
		machine.Log("error while setting up: "+err.Error(), Critical)
		return
	}
	err = machine.containers[0].Create()
	if err != nil {
		machine.Log("error while creating: "+err.Error(), Critical)
		return
	}
	machine.containers[0].Start()

}
