package stream

import (
	"context"
	"errors"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"supervisor/machine/container/listener/event"
	"sync"
	"time"
)

type Stream struct {
	LastRead time.Time
	Open     bool
	Mutex    sync.Mutex
	Cancel   context.CancelFunc
	Ctx      context.Context

	// id
	ContainerName string
	ContainerId   string
	Type          event.Type

	// global
	Client *client.Client

	HandlerEvents *chan event.Entry
}

func (s *Stream) logger() (logger *log.Entry) {
	return log.WithFields(log.Fields{
		"handler": s.ContainerId,
		"open":    s.Open,
		"type":    s.Type,
	})
}

/*
*
will perform initial checks and set the initial log start time when needed.
this should be implemented with a mutex lock before and after, including this
function call, as well as the method opening the actual stream before the
mutex release
*/
func (s *Stream) PreCheck() (err error) {

	if s.Client == nil {
		err = errors.New("nil client")
		s.logger().Error("nil client while streaming")
		return err
	}
	if s.Open {
		err = errors.New("already open")
		s.logger().Error("already streaming")
		return err
	}
	if s.LastRead.IsZero() {
		s.LastRead = time.Now()
		s.logger().Info("setting last read to current time")
	}

	s.Ctx, s.Cancel = context.WithCancel(context.Background())

	return err
}

func (s *Stream) Close() {
	if s.Cancel != nil {
		s.Cancel()
	}
}
