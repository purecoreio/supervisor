package machine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net"
	"net/url"
	"os"
	"strconv"
	"supervisor/machine/container"
	"supervisor/machine/proto"
	"time"
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
	addr    = flag.String("addr", "hansel.serverbench.io", "http service address")
	ctx     = context.Background()
)

type Machine struct {
	Directory string // /etc/serverbench/supervisor/containers/
	Group     string // serverbench
	conn      *websocket.Conn
	cli       *client.Client
}

func (m Machine) Init(try int) (err error) {
	m.logger().Info("preparing connection")
	if try > 13 {
		try -= 1
	}
	time.Sleep(time.Second * time.Duration(try*5))
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err == nil {
		m.cli = cli
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
										Rid:   message.Rid,
										Type:  "password",
										Data:  *pswd,
										Error: false,
									}
								}
								break
							}
						case "logs":
							{
								go func() {
									target.GetStream(message.Rid, m.conn).StreamLogs(m.cli)
								}()
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
				isError := false
				if err != nil {
					isError = true
					m.logger().Info("sending errored ack: " + err.Error())
				} else {
					m.logger().Info("sending ok ack")
				}
				err = m.conn.WriteJSON(proto.Response{
					Rid:   message.Rid,
					Type:  "ack",
					Error: isError,
				})
				if err != nil {
					m.logger().Error("error while encoding ack: " + err.Error())
					continue
				}
				m.logger().Info("sent ack: " + message.Rid)

				if reply != nil {
					err := m.conn.WriteJSON(reply)
					if err != nil {
						m.logger().Error("error while response for " + message.Rid + ": " + err.Error())
						continue
					} else {
						m.logger().Info("sent reply for " + message.Rid)
					}
				}
			}
		}
		if err == nil {
			m.logger().Info("socket closed")
		} else {
			m.logger().Error("socket closed " + err.Error())
		}
	} else {
		m.logger().Error("error while getting docker: " + err.Error())
	}
	return m.Init(try)
}

func (m Machine) getLoginString() (params url.Values, err error) {

	// 1. hostname
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	// 2. token
	token, err := m.GetToken()
	if err != nil {
		return nil, err
	}

	// 3. networking
	var inets []container.Ip // list of non-empty network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, inet := range interfaces {
		addressList, err := inet.Addrs()
		if err != nil {
			return nil, err
		}
		for _, inetAddr := range addressList {
			ip, _, err := net.ParseCIDR(inetAddr.String())
			if err != nil {
				return nil, err
			}

			if !ip.IsLoopback() {
				inets = append(inets, container.Ip{
					Ip:        inetAddr.String(),
					Adapter:   inet.Name,
					Available: true,
				})
			}
		}
	}
	m.logger().Info("found addresses: " + strconv.Itoa(len(inets)))
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
	return log.WithFields(log.Fields{}) // TODO explore
}

func (m Machine) Host(container container.Container) (err error) {
	m.logger().Info("hosting " + container.Id)
	_, err = m.createUser(container.Username(), container.Path)
	if err != nil {
		return err
	}

	// container should mount volume onto settings.path/data
	go func() {
		_ = container.Start(m.cli, ctx)
	}()
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
