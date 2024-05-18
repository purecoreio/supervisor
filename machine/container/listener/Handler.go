package listener

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type Handler struct {
	Status   []Subscriber
	Logs     []Subscriber
	Progress []Subscriber
	// global
	client *client.Client
	Out    *chan []byte
	// id
	ContainerId   string
	ContainerName string
	// logs
	LastLogRead time.Time
	LogOpen     bool
	StopChan    chan os.Signal
}

func (h Handler) logger() (entry *log.Entry) {
	return log.WithFields(log.Fields{
		"handler": h.ContainerId,
	})
}

func (h Handler) Init(out *chan []byte, id string, name string, client *client.Client) {
	h.Out = out
	h.client = client
	h.Status = make([]Subscriber, 0)
	h.Logs = make([]Subscriber, 0)
	h.Progress = make([]Subscriber, 0)
	h.ContainerId = id
	h.ContainerName = name
	h.StopChan = make(chan os.Signal, 1)
}

func (h Handler) Subscribe(listener Subscriber) (err error) {
	if h.Out == nil {
		err = errors.New("missing out channel")
		return err
	}
	if listener.Level.Status {
		h.Status = append(h.Status, listener)
	}
	if listener.Level.Progress {
		h.Progress = append(h.Progress, listener)
	}
	if listener.Level.Logs {
		if !listener.Level.Status {
			// when a container stops, the log stream stops, so we need to be on the lookout for restart events
			// to re-attach to the container logs
			err = errors.New("in order to listen for log events, you must also attach to status events")
			return err
		}
		h.Logs = append(h.Logs, listener)
		if !h.LogOpen {
			func() {
				err := h.streamLogs()
				if err != nil {
					return
				}
			}()
		}
	}
	return err
}

func (h Handler) Unsubscribe(subscriber Subscriber) (err error) {
	_, err = h.cleanSubscriberList(subscriber, &h.Status)
	if err != nil {
		return err
	}
	empty, err := h.cleanSubscriberList(subscriber, &h.Logs)
	if err != nil {
		return err
	}
	if empty {
		h.StopChan <- syscall.SIGTERM
	}

	_, err = h.cleanSubscriberList(subscriber, &h.Progress)
	if err != nil {
		return err
	}
	return err
}

func (h Handler) cleanSubscriberList(subscriber Subscriber, subscriberList *[]Subscriber) (empty bool, err error) {
	if subscriberList == nil {
		err = errors.New("invalid subscriber list")
	} else {
		finalSubscribers := make([]Subscriber, 0)
		for _, listedSubscriber := range *subscriberList {
			if listedSubscriber.Id != subscriber.Id {
				finalSubscribers = append(finalSubscribers, listedSubscriber)
			}
		}
		if len(finalSubscribers) <= 0 {
			empty = true
		}
		*subscriberList = finalSubscribers
	}
	return empty, err
}

func (h Handler) HandleEvent(action Type, content string) (err error) {
	h.logger().Info("got event %s, %s", action, content)
	var targetListeners *[]Subscriber = nil
	if action == Log {
		targetListeners = &h.Logs
	} else if action == Status {
		targetListeners = &h.Status
	} else if action == Progress {
		targetListeners = &h.Progress
	} else {
		err = errors.New("unknown action")
		return err
	}
	for _, targetListener := range *targetListeners {
		event := Event{
			Listener:  targetListener.Id,
			Type:      action,
			Container: h.ContainerId,
			Content:   content,
		}
		if event.Type == Status && event.Content == "start" && !h.LogOpen && len(h.Logs) > 0 {
			err := h.streamLogs()
			if err != nil {
				return err
			}
		}
		bytes, err := json.Marshal(event)
		if err != nil {
			return err
		}
		h.logger().Info("sending event", event)
		*h.Out <- bytes
	}
	return err
}

// logs
/**
IO blocking log streaming
*/
func (h Handler) streamLogs() (err error) {
	if h.client == nil {
		err = errors.New("nil client")
		return err
	}
	if h.LogOpen {
		err = errors.New("logs already open")
		return err
	}
	if h.LastLogRead.IsZero() {
		h.LastLogRead = time.Now()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logs, err := h.client.ContainerLogs(ctx, h.ContainerName, container.LogsOptions{
		Follow:     true,
		Since:      strconv.FormatInt(h.LastLogRead.Unix(), 10),
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return err
	}
	defer logs.Close()
	done := make(chan struct{})
	signal.Notify(h.StopChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		h.LogOpen = true
		buffer := make([]byte, 4096)
		for {
			n, err := logs.Read(buffer)
			if n > 0 {
				err := h.HandleEvent(Log, string(buffer[:n]))
				if err != nil {
					break
				}
			}
			if err == io.EOF {
				break
			}
		}
		close(done)
	}()

	select {
	case <-h.StopChan:
		cancel()
		<-done
	case <-done:
	}

	h.LastLogRead = time.Now()
	h.LogOpen = false
	cancel()
	return nil
}
