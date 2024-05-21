package stream

import (
	"github.com/docker/docker/api/types/container"
	"io"
	"strconv"
	"supervisor/machine/container/listener/event"
	"time"
)

func (s *Stream) StreamLogs() (err error) {
	s.logger().Info("starting log stream")
	s.Mutex.Lock()
	err = s.PreCheck()
	if err != nil {
		return err
	}
	logs, err := s.Client.ContainerLogs(s.Ctx, s.ContainerName, container.LogsOptions{
		Follow:     true,
		Since:      strconv.FormatInt(s.LastRead.Unix(), 10),
		ShowStdout: true,
		ShowStderr: true,
	})
	if err == nil {
		s.Open = true
	}
	s.Mutex.Unlock()
	if err != nil {
		s.logger().Error("error while opening log stream")
		return err
	}

	buffer := make([]byte, 4096)

	defer func() {
		_ = logs.Close()
		s.Mutex.Lock()
		s.LastRead = time.Now()
		s.Open = false
		s.Cancel = nil
		s.Ctx = nil
		s.Mutex.Unlock()
		s.logger().Info("log stream ended")
	}()

	for {
		select {
		case <-s.Ctx.Done():
			return nil
		default:
			n, err := logs.Read(buffer)
			if n > 0 {
				*s.HandlerEvents <- event.Entry{
					Type:    event.Log,
					Content: string(buffer[:n]),
				}
			}
			if err == io.EOF {
				return nil
			}
		}
	}

}
