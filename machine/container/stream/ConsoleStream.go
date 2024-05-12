package stream

import (
	"bufio"
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func (s Stream) StreamLogs(cli *client.Client) {
	if s.prepare() {
		s.logger().Info("starting log stream")
		err := s.unsafeStreamLogs(cli)
		s.logger().Info("stream stopped: " + err.Error())
		s.Kill()
	}
}

func (s Stream) unsafeStreamLogs(cli *client.Client) (err error) {
	err = s.Check()
	if err != nil {
		return err
	}

	options := container.LogsOptions{
		ShowStdout: true,
		Follow:     true,
		Timestamps: false,
		Tail:       "all",
	}
	reader, err := cli.ContainerLogs(context.Background(), s.Cid, options)
	if err != nil {
		return err
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		msg, err := s.prepareMessage(scanner.Text())
		if err != nil {
			return err
		}
		err = s.Conn.WriteJSON(msg)
		if err != nil {
			return err
		}
	}
	return err
}
