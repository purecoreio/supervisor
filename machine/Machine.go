package machine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"github.com/docker/docker/api/types"
	dContainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"supervisor/machine/container"
	"supervisor/machine/container/listener"
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
	Directory  string // /etc/serverbench/supervisor/containers/
	Group      string // serverbench
	Containers map[string]container.Container
	events     chan []byte
	conn       *websocket.Conn
	cli        *client.Client
}

func (m Machine) Init(try int) (err error) {
	// don't allow tries to go increase wait times infinitely
	if try > 12 {
		try -= 1
	} else if try > 1 {
		time.Sleep(time.Second * time.Duration(try*5))
	}
	m.events = make(chan []byte)
	// init cli
	m.cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		m.logger().Error("unable to get docker client", err)
		return m.Init(try + 1)
	}
	// list local containers
	m.Containers, err = m.loadContainersFromDocker()
	if err != nil {
		m.logger().Error("unable to list hosted containers locally")
		return m.Init(try + 1)
	}
	// events
	err = m.listenForEvents()
	if err != nil {
		m.logger().Error("unable to list hosted containers locally")
		return m.Init(try + 1)
	}
	// connect
	params, err := m.getLoginString()
	if err != nil {
		m.logger().Error("unable to get login string", err)
		return m.Init(try + 1)
	}
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/machine"}
	u.RawQuery = params.Encode()
	m.logger().Info("connecting")
	dial, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		m.logger().Error("unable to get dial socket", err)
		return m.Init(try + 1)
	}
	m.logger().Info("connected")
	try = 0
	m.conn = dial
	// handle events
	for {
		_, in, err := m.conn.ReadMessage()
		if err != nil {
			m.logger().Error("unable to read message (socket likely closed)", err)
			break
		}
		message := proto.Message{}
		err = json.Unmarshal(bytes.TrimSpace(bytes.Replace(in, newline, space, -1)), &message)
		if err != nil {
			m.logger().Warn("malformed message received ("+string(in)+")", err.Error())
			continue
		}
		m.logger().Info("received request: %v", message)
		reply, err := m.handleMessage(message)
		err = m.conn.WriteJSON(proto.Response{
			Rid:   message.Rid,
			Type:  "ack",
			Error: err != nil,
		})
		if err != nil {
			m.logger().Warn("error while encoding ack: " + err.Error())
			continue
		}
		if reply != nil {
			err := m.conn.WriteJSON(reply)
			if err != nil {
				m.logger().Warn("error while replying %s: %v", message.Rid, err)
				continue
			}
		}
		m.logger().Info("fulfilled request: %v", reply)
	}
	return m.Init(try + 1)
}

func (m Machine) loadContainersFromDocker() (containers map[string]container.Container, err error) {
	containers = make(map[string]container.Container)
	if m.cli == nil {
		err = errors.New("invalid cli")
		return containers, err
	}
	dContainers, err := m.cli.ContainerList(ctx, dContainer.ListOptions{
		All: true,
	})
	if err != nil {
		return containers, err
	}
	prefix := "/sb-"
	for _, c := range dContainers {
		for _, name := range c.Names {
			if strings.HasPrefix(name, prefix) {
				containerId := name[len(prefix):]
				containers[containerId] = container.Container{
					Id: containerId,
				}
				containers[containerId].Handler.Init(&m.events, containerId, m.Containers[containerId].Username(), m.cli)
				m.logger().Infof("loaded container %s", containerId)
			}
		}
	}
	return containers, err
}

func (m Machine) listenForEvents() (err error) {
	if m.cli == nil {
		return errors.New("missing cli")
	}
	if m.events == nil {
		return errors.New("missing events")
	}
	m.logger().Info("starting event listener")
	msg, errs := m.cli.Events(context.Background(), types.EventsOptions{
		Filters: filters.NewArgs(
			filters.Arg("type", "container"),
			filters.Arg("event", "die"),
			filters.Arg("event", "pause"),
			filters.Arg("event", "restart"),
			filters.Arg("event", "start"),
			filters.Arg("event", "stop"),
		),
	})
	go func() {
		for {
			select {
			case event := <-msg:
				name := event.Actor.Attributes["name"]
				if !strings.HasPrefix(name, "sb-") {
					continue
				}
				containerId := name[3:]
				localContainer, ok := m.Containers[containerId]
				if !ok {
					continue
				}
				err := localContainer.Handler.HandleEvent(listener.Log, string(event.Action))
				if err != nil {
					m.logger().Warn("error while handling event: %s", err)
					continue
				}
			case err := <-errs:
				m.logger().Error("event listener got an error", err.Error())
				time.Sleep(1 * time.Second)
				m.listenForEvents()
			}
		}
	}()
	return err
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
