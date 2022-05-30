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
				path: "/home/quiquelhappy/containers",
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
					cores:  0.1,
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
	machine.containers[0].machine = &machine
	machine.storage[0].machine = &machine
	machine.containers[0].storage = &machine.storage[0]
	err := machine.setupSSHD()
	if err != nil {
		fmt.Println("error while setting up sshd: " + err.Error())
		return
	}
	password, err := machine.containers[0].CreateUser()
	if err != nil {
		fmt.Println("error while creating user: " + err.Error())
		return
	}
	fmt.Println("created user " + machine.containers[0].id + " with password " + password)
	/*
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
		machine.containers[0].Start()*/

}
