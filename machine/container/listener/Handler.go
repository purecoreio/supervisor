package listener

import (
	"context"
	"errors"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

type Handler struct {
	Status   []Subscriber
	Logs     []Subscriber
	Progress []Subscriber
	// global
	client *client.Client
	Out    *chan Event
	// id
	ContainerId   string
	ContainerName string
	// logs
	LastLogRead time.Time
	LogOpen     atomic.Bool
	StopChan    chan os.Signal
}

func (h *Handler) logger() (entry *log.Entry) {
	return log.WithFields(log.Fields{
		"handler":  h.ContainerId,
		"logs":     len(h.Logs),
		"progress": len(h.Progress),
		"status":   len(h.Status),
	})
}

func (h *Handler) Init(out *chan Event, id string, name string, client *client.Client) {
	h.Out = out
	h.client = client
	h.Status = make([]Subscriber, 0)
	h.Logs = make([]Subscriber, 0)
	h.Progress = make([]Subscriber, 0)
	h.ContainerId = id
	h.ContainerName = name
	h.StopChan = make(chan os.Signal, 1)
	h.LogOpen.Store(false)
}

func (h *Handler) Subscribe(listener Subscriber) (err error) {
	h.logger().Info("subscribing ", listener)
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
		} else {
			h.Logs = append(h.Logs, listener)
			if !h.LogOpen.Load() {
				err = h.streamLogs()
			}
		}
	}
	h.logger().Info("subscribed ", listener)
	return err
}

func (h *Handler) Unsubscribe(subscriber Subscriber) (err error) {
	_, err = h.cleanSubscriberList(subscriber, &h.Status)
	if err != nil {
		return err
	}
	empty, err := h.cleanSubscriberList(subscriber, &h.Logs)
	if err != nil {
		return err
	}
	if empty && h.LogOpen.Load() {
		h.logger().Info("stopping log stream")
		h.StopChan <- syscall.SIGTERM
	}

	_, err = h.cleanSubscriberList(subscriber, &h.Progress)
	if err != nil {
		return err
	}
	return err
}

func (h *Handler) cleanSubscriberList(subscriber Subscriber, subscriberList *[]Subscriber) (empty bool, err error) {
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

func (h *Handler) HandleEvent(action Type, content string) (err error) {
	var targetListeners *[]Subscriber = nil
	if action == Log {
		targetListeners = &h.Logs
	} else if action == Status {
		targetListeners = &h.Status
	} else if action == Progress {
		targetListeners = &h.Progress
	} else {
		err = errors.New("unknown action")
		h.logger().Errorf("unknown event %s, %s", action, content)
		return err
	}
	for _, targetListener := range *targetListeners {
		event := Event{
			Listener:  targetListener.Id,
			Type:      action,
			Container: h.ContainerId,
			Content:   content,
		}
		if event.Type == Status && event.Content == "start" && !h.LogOpen.Load() && len(h.Logs) > 0 {
			err := h.streamLogs()
			if err != nil {
				h.logger().Errorf("error starting logs after receiving %s, %s", action, content)
				return err
			}
		}
		*h.Out <- event
		h.logger().Infof("forwarded event %s, %s", action, content)
	}
	return err
}

// logs
/**
IO blocking log streaming
*/
func (h *Handler) streamLogs() (err error) {
	h.logger().Info("starting log stream")
	if h.client == nil {
		err = errors.New("nil client")
		h.logger().Error("nil client while streaming logs")
		return err
	}
	if h.LogOpen.Load() {
		err = errors.New("logs already open")
		h.logger().Error("already streaming logs")
		return err
	}
	if h.LastLogRead.IsZero() {
		h.LastLogRead = time.Now()
		h.logger().Info("setting last log read to current time")
	}
	logs, err := h.client.ContainerLogs(context.Background(), h.ContainerName, container.LogsOptions{
		Follow:     true,
		Since:      strconv.FormatInt(h.LastLogRead.Unix(), 10),
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		h.logger().Error("error while opening log stream")
		return err
	}

	h.LogOpen.Store(true)

	// Move the log reading part to a new goroutine
	go func() {

		buffer := make([]byte, 4096)

		defer logs.Close()

	logStream:
		for {
			select {
			case <-h.StopChan:
				{
					h.logger().Info("log stream stopped")
					break logStream
				}
			default:
				{
					n, err := logs.Read(buffer)
					if n > 0 {
						err := h.HandleEvent(Log, string(buffer[:n]))
						if err != nil {
							break logStream
						}
					}
					if err == io.EOF {
						break logStream
					}
				}
			}
		}

		h.logger().Info("log stream ended")
		h.LastLogRead = time.Now()
		h.LogOpen.Store(false)
	}()

	return nil
}
