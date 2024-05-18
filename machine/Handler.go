package machine

import (
	"encoding/json"
	"errors"
	"supervisor/machine/container"
	"supervisor/machine/container/listener"
	"supervisor/machine/proto"
)

func (m Machine) handleMessage(message proto.Message) (reply *proto.Response, err error) {
	switch message.Realm {
	case "machine":
		{
			switch message.Command {
			case "handshake":
				{
					subscriber := listener.Subscriber{}
					err = json.Unmarshal([]byte(*message.Data), &subscriber)
					if subscriber.Containers == nil {
						err = errors.New("you must provide a container list")
					}
					if err == nil {
						for _, containerId := range *subscriber.Containers {
							_, ok := m.Containers[containerId]
							if !ok {
								err = errors.New("container not found")
								return nil, err
							}
							err = m.Containers[containerId].Handler.Subscribe(subscriber)
						}
					}
					break
				}
			case "farewell":
				{
					subscriber := listener.Subscriber{}
					err = json.Unmarshal([]byte(*message.Data), &subscriber)
					if err == nil {
						for _, presentContainer := range m.Containers {
							err := presentContainer.Handler.Unsubscribe(subscriber)
							if err != nil {
								break
							}
						}
					}
					break
				}
			}
			break
		}
	case "container":
		{
			target, ok := m.Containers[*message.Target]
			if !ok {
				if message.Command == "host" {
					target = container.Container{
						Id: *message.Target,
					}
					err = json.Unmarshal([]byte(*message.Data), &target)
					if err != nil && target.Id != *message.Target {
						err = errors.New("target mismatch (while creating new container)")
					} else {
						target.Handler.Init(&m.events, target.Id, target.Username(), m.cli)
					}
				} else {
					err = errors.New("you must provide a hosted container id (unless you are hosting a new container)")
				}
			}
			if err != nil {
				return nil, err
			}
			switch message.Command {
			case "host":
				{
					err = m.Host(target)
					break
				}
			case "delete":
				{
					err = m.Unhost(target)
					break
				}
			case "password":
				{
					pswd, err := m.resetPassword(target.Username())
					if err == nil {
						reply = &proto.Response{
							Rid:   message.Rid,
							Type:  "password",
							Data:  *pswd,
							Error: false,
						}
					}
					break
				}
			// power
			case "start":
				{
					break
				}
			case "stop":
				{
					break
				}
			case "restart":
				{
					break
				}
			case "pause":
				{
					break
				}
			case "unpause":
				{
					break
				}
			case "exec":
				{
					break
				}
			}
			break
		}
	}
	return reply, err
}
