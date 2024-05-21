package stream

import (
	"encoding/json"
	"github.com/docker/docker/api/types"
	"io"
	"supervisor/machine/container/listener/event"
	"time"
)

func (s *Stream) StreamLoad() (err error) {
	s.logger().Info("starting load stream")
	s.Mutex.Lock()
	err = s.PreCheck()
	if err != nil {
		return err
	}
	load, err := s.Client.ContainerStats(s.Ctx, s.ContainerName, true)
	if err == nil {
		s.Open = true
	}
	s.Mutex.Unlock()
	if err != nil {
		s.logger().Error("error while opening load stream")
		return err
	}

	reader := load.Body
	buffer := make([]byte, 4096)

	defer func() {
		_ = reader.Close()
		s.Mutex.Lock()
		s.LastRead = time.Now()
		s.Open = false
		s.Cancel = nil
		s.Ctx = nil
		s.Mutex.Unlock()
		s.logger().Info("load stream ended")
	}()

	for {
		select {
		case <-s.Ctx.Done():
			return nil
		default:
			n, err := reader.Read(buffer)
			if n > 0 {
				// TODO parse stat data
				stat := types.Stats{}
				parseErr := json.Unmarshal(buffer[:n], &stat)
				if parseErr != nil {
					s.logger().Warn("error while unmarshalling stats")
					s.logger().Warn(parseErr)
				} else if stat.CPUStats.OnlineCPUs != 0 {
					*s.HandlerEvents <- event.Entry{
						Type:    event.Load,
						Content: string(buffer[:n]),
					}
				} else {
					s.logger().Info("container appears offline, detaching load")
					return nil
				}
			}
			if err == io.EOF {
				return nil
			}
		}
	}
}
