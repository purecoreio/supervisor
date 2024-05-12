package stream

import (
	"errors"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"supervisor/machine/proto"
)

type Stream struct {
	Path   string
	Id     string
	Cid    string
	Rid    string
	closed bool
	Conn   *websocket.Conn
}

func (s Stream) logger() (entry *log.Entry) {
	return log.WithFields(log.Fields{
		"stream":    s.Rid,
		"container": s.Id,
	})
}

func (s Stream) prepare() (start bool) {
	s.logger().Info("starting stream")
	startMsg := proto.StreamMessage{
		Rid:    s.Rid,
		Stream: true,
		Start:  true,
	}
	err := s.Check()
	if err == nil {
		err = s.Conn.WriteJSON(startMsg)
		if err == nil {
			// stream has now started
			s.logger().Info("stream started")
			return true
		} else {
			s.logger().Error("unable to write starting message: " + err.Error())
		}
	} else {
		s.logger().Error("did not pass initial check: " + err.Error())
	}
	return false
}

func (s Stream) prepareMessage(data string) (message proto.StreamMessage, err error) {
	err = s.Check()
	if err != nil {
		return message, err
	}
	message = proto.StreamMessage{
		Rid:    s.Rid,
		Stream: true,
		Data:   data,
	}
	return message, nil
}

func (s Stream) Check() (err error) {
	if s.closed {
		return errors.New("stream is closed")
	}
	return nil
}

func (s Stream) Kill() {
	if s.closed {
		s.logger().Error("tried to kill stream, but it was already killed")
		return
	}
	s.logger().Info("killing stream")
	err := s.Check()
	s.closed = true
	if err == nil {
		_ = s.Conn.WriteJSON(proto.StreamMessage{
			Rid:    s.Rid,
			Stream: true,
			End:    true,
		})
	} else {
		s.logger().Error("unable to send stream end message: " + err.Error())
	}
	s.logger().Info("killed stream")
}
