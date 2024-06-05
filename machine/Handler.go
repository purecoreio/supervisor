package machine

import (
	"encoding/json"
	"errors"
	"supervisor/machine/container"
	"supervisor/machine/container/listener"
	"supervisor/machine/proto/in"
	"supervisor/machine/proto/out"
)

func (m *Machine) handleMessage(message in.Message) (reply *out.Response, err error) {
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
							if err != nil {
								return nil, err
							}
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
			if message.Command == "host" {
				target = container.Container{
					Id: *message.Target,
				}
				hostRequest := in.HostRequest{}
				err = json.Unmarshal([]byte(*message.Data), &hostRequest)
				if err != nil && hostRequest.Container.Id != *message.Target {
					err = errors.New("target mismatch (while creating new container)")
				} else {
					target = hostRequest.Container
					// TODO keep handler working between installs
					err := target.Init(&m.events, m.cli)
					if err != nil {
						return nil, err
					}
				}
			} else if !ok {
				err = errors.New("you must provide a hosted container id (unless you are hosting a new container)")
			}
			if err != nil {
				return nil, err
			}
			switch message.Command {
			case "host":
				{
					hostRequest := in.HostRequest{}
					err = json.Unmarshal([]byte(*message.Data), &hostRequest)
					err = target.Host(m.cli, m.Containers, hostRequest.Token, hostRequest.HeadSha)
					break
				}
			case "delete":
				{
					err = target.Unhost(m.cli, m.Containers)
					break
				}
			case "password":
				{
					pswd, err := target.ResetPassword()
					if err == nil {
						reply = &out.Response{
							Rid:  message.Rid,
							Type: "password",
							Data: out.PasswordResponse{
								Password: *pswd,
							},
							Error: false,
						}
					}
					break
				}
			case "reset_key":
				{
					key, err := target.ResetKeys()
					if err == nil {
						reply = &out.Response{
							Rid:  message.Rid,
							Type: "key",
							Data: out.PublicKeyResponse{
								Key: key,
							},
							Error: false,
						}
					}
					break
				}
			case "public_key":
				{
					key, err := target.GetPublicKey()
					if err == nil {
						reply = &out.Response{
							Rid:  message.Rid,
							Type: "key",
							Data: out.PublicKeyResponse{
								Key: key,
							},
							Error: false,
						}
					}
					break
				}
			case "list_authorized_keys":
				{
					keys, err := target.ListAuthorizedKeys()
					if err == nil {
						reply = &out.Response{
							Rid:  message.Rid,
							Type: "authorized_keys",
							Data: out.KeyListResponse{
								Keys: keys,
							},
							Error: false,
						}
					}
					break
				}
			case "authorize_key":
				{
					keyRequest := in.KeyRequest{}
					err = json.Unmarshal([]byte(*message.Data), &keyRequest)
					err = target.AddAuthorizedKey(keyRequest.Key)
					break
				}
			case "deauthorize_key":
				{
					keyRequest := in.KeyRequest{}
					err = json.Unmarshal([]byte(*message.Data), &keyRequest)
					err = target.RemoveAuthorizedKey(keyRequest.Key)
					break
				}
			case "transfer":
				{
					transferRequest := in.TransferRequest{}
					err = json.Unmarshal([]byte(*message.Data), &transferRequest)
					go func() {
						_ = target.Transfer(transferRequest.Out, transferRequest.Address, transferRequest.Port, transferRequest.Path, transferRequest.User, transferRequest.Mirror, transferRequest.Password, nil)
					}()
					break
				}
			// power
			case "start":
				{
					err = target.Start(m.cli, nil, nil)
					break
				}
			case "stop":
				{
					err = target.Stop(m.cli)
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
