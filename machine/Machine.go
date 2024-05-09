package machine

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net"
	"net/url"
	"os"
	"slices"
	"supervisor/machine/container"
	"supervisor/machine/proto"
	"time"
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
	addr    = flag.String("addr", "hansel.serverbench.io", "http service address")
)

type Machine struct {
	Id        string
	Directory string // /etc/serverbench/supervisor/containers/
	Group     string // serverbench
	conn      *websocket.Conn
}

func (m Machine) Init(token string, try int) (err error) {
	m.logger().Info("preparing connection")
	if try > 13 {
		try -= 1
	}
	time.Sleep(time.Second * time.Duration(try*5))
	try += 1
	params, err := m.getLoginString()
	if err != nil {
		return err
	}
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/machine"}
	u.RawQuery = params.Encode()
	m.logger().Info("connecting")
	dial, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err == nil {
		m.logger().Info("connected")
		try = 0
		m.conn = dial
		defer func() {
			m.conn.Close()
		}()

		for {
			_, in, err := m.conn.ReadMessage()
			if err != nil {
				// socket closed, reconnect
				break
			}
			message := proto.Message{}
			err = json.Unmarshal(bytes.TrimSpace(bytes.Replace(in, newline, space, -1)), &message)
			if err != nil {
				m.logger().Error("malformed message received (" + string(in) + ") " + err.Error())
				continue
			}
			var reply *proto.Response = nil
			switch message.Realm {
			case "container":
				{
					target := container.Container{
						Id: *message.Target,
					}
					switch message.Command {
					// host, unhost, update, password
					case "host":
						{
							var newTarget = container.Container{}
							err = json.Unmarshal([]byte(*message.Data), &newTarget)
							if err == nil {
								if newTarget.Id != target.Id {
									err = errors.New("id mismatch")
								} else {
									target = newTarget
									err = m.Host(target)
								}
							}
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
									Rid:     message.Rid,
									Content: pswd,
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
					// monitoring
					case "status":
						{
							break
						}
					case "logs":
						{
							break
						}
					case "exec":
						{
							break
						}
					case "detach":
						{
							break
						}
					}
					break
				}
			}
			if err == nil {
				reply = &proto.Response{
					Rid: message.Rid,
				}
			} else {
				reply = &proto.Response{
					Rid:   message.Rid,
					Error: true,
				}
				m.logger().Warn("forwarding error: " + err.Error())
			}
			encodedResponse, err := json.Marshal(reply)
			if err == nil {
				m.logger().Info("reply " + string(encodedResponse))
			} else {
				m.logger().Error("error while encoding response: " + err.Error())
			}
		}
	}
	if err == nil {
		m.logger().Info("socket closed")
	} else {
		m.logger().Error("socket closed " + err.Error())
	}
	return m.Init(token, try)
}

func (m Machine) getLoginString() (params url.Values, err error) {

	// 1. hostname
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	// 2. token
	token, err := m.getToken()
	if err != nil {
		return nil, err
	}

	// 3. networking
	var inets []proto.Inet // list of non-empty network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, inet := range interfaces {
		addressList, err := inet.Addrs()
		if err != nil {
			return nil, err
		}
		var validIps []string
		for _, inetAddr := range addressList {
			ip, _, err := net.ParseCIDR(inetAddr.String())
			if err != nil {
				return nil, err
			}

			if !ip.IsLoopback() {
				slices.Insert(validIps, len(validIps), ip.String())
			}
		}
		if len(validIps) > 0 {
			slices.Insert(inets, len(inets), proto.Inet{
				Name: inet.Name,
				Addr: validIps,
			})
		}
	}
	serializedInets, err := json.Marshal(inets)
	if err != nil {
		return nil, err
	}
	params = url.Values{}
	params.Set("token", *token)
	params.Set("hostname", hostname)
	params.Set("inets", string(serializedInets)) // should escape URL formatting
	return params, err
}

func (m Machine) logger() (entry *log.Entry) {
	return log.WithFields(log.Fields{
		"machine": m.Id,
	})
}

func (m Machine) Host(container container.Container) (err error) {
	m.logger().Info("hosting " + container.Id)
	_, err = m.createUser(container.Username(), container.Settings.Path)
	if err != nil {
		return err
	}

	// container should mount volume onto settings.path/data
	m.logger().Info("hosted " + container.Id)
	return nil
}

func (m Machine) Unhost(container container.Container) (err error) {
	m.logger().Info("unhosting " + container.Id)
	err = m.removeUser(container.Username())
	if err != nil {
		return err
	}
	m.logger().Info("unhosted " + container.Id)
	return nil
}
