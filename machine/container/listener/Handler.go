package listener

import (
	"context"
	"errors"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"supervisor/machine/container/listener/event"
	"supervisor/machine/container/listener/stream"
)

type Handler struct {
	Status   []Subscriber
	Logs     []Subscriber
	Progress []Subscriber
	Load     []Subscriber
	// global
	Client *client.Client
	// id
	ContainerId   string
	ContainerName string
	// local event stream
	internalEvents *chan event.Entry
	eventPool      *chan event.Entry
	// logs
	LogStream *stream.Stream
	// load
	LoadStream *stream.Stream
}

var (
	MissingStatusErr = errors.New("in order to listen for log/load events, you must also attach to status events")
)

/*
*
create streams and forward events
*/
func (h *Handler) Forward(Out *chan event.Entry) (err error) {
	if h.internalEvents != nil {
		err = errors.New("already forwarding events")
		return err
	}
	if Out == nil {
		err = errors.New("missing event pool")
		return err
	}
	h.eventPool = Out
	internal := make(chan event.Entry)
	h.internalEvents = &internal
	logStream := stream.Stream{
		Client:        h.Client,
		HandlerEvents: h.internalEvents,
		ContainerName: h.ContainerName,
		ContainerId:   h.ContainerId,
		Type:          event.Log,
	}
	h.LogStream = &logStream
	loadStream := stream.Stream{
		Client:        h.Client,
		HandlerEvents: h.internalEvents,
		ContainerName: h.ContainerName,
		ContainerId:   h.ContainerId,
		Type:          event.Load,
	}
	h.LoadStream = &loadStream
	go func() {
		for entry := range *h.internalEvents {
			err = h.HandleEvent(entry.Type, entry.Content)
		}
	}()
	return err
}

func (h *Handler) logger() (entry *log.Entry) {
	return log.WithFields(log.Fields{
		"handler":  h.ContainerId,
		"logs":     len(h.Logs),
		"progress": len(h.Progress),
		"status":   len(h.Status),
		"load":     len(h.Load),
	})
}

func (h *Handler) Subscribe(listener Subscriber) (err error) {
	h.logger().Info("subscribing ", listener)
	if h.internalEvents == nil {
		err = errors.New("missing out channel")
		return err
	}
	var status string
	if listener.Level.Status {
		h.Status = append(h.Status, listener)
		inspect, err := h.Client.ContainerInspect(context.Background(), h.ContainerName)
		if err != nil {
			return err
		}
		status = inspect.State.Status
		err = h.HandleEvent(event.Status, status)
		if err != nil {
			return err
		}
	}
	if listener.Level.Progress {
		h.Progress = append(h.Progress, listener)
	}
	if listener.Level.Logs {
		if !listener.Level.Status {
			// when a container stops, the log stream stops, so we need to be on the lookout for restart events
			// to re-attach to the container logs
			err = MissingStatusErr
		} else {
			h.Logs = append(h.Logs, listener)
			go func() {
				_ = h.LogStream.StreamLogs()
			}()
		}
	}
	if listener.Level.Load {
		if !listener.Level.Status {
			// when a container stops, the load stream stops, so we need to be on the lookout for restart events
			// to re-attach to the container load
			err = MissingStatusErr
		} else {
			h.Load = append(h.Load, listener)
			go func() {
				_ = h.LoadStream.StreamLoad()
			}()
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
	if empty {
		h.LogStream.Close()
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

func (h *Handler) HandleEvent(action event.Type, content string) (err error) {
	var targetListeners *[]Subscriber = nil
	if action == event.Log {
		targetListeners = &h.Logs
	} else if action == event.Status {
		targetListeners = &h.Status
	} else if action == event.Progress {
		targetListeners = &h.Progress
	} else if action == event.Load {
		targetListeners = &h.Load
	} else {
		err = errors.New("unknown action")
		h.logger().Errorf("unknown event %s, %s", action, content)
		return err
	}
	for _, targetListener := range *targetListeners {
		entry := event.Entry{
			Listener:  targetListener.Id,
			Type:      action,
			Container: h.ContainerId,
			Content:   content,
		}
		if entry.Type == event.Status && entry.Content == "start" {
			if len(h.Logs) > 0 {
				go func() {
					_ = h.LogStream.StreamLogs()
				}()
			}
			if len(h.Load) > 0 {
				go func() {
					_ = h.LoadStream.StreamLoad()
				}()
			}
		}
		*h.eventPool <- entry
		h.logger().Infof("forwarded entry %s, %s", action, content)
	}
	return err
}
