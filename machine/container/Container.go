package container

import (
	log "github.com/sirupsen/logrus"
)

type Container struct {
	Id       string    `json:"id"`
	Template Template  `json:"template"`
	Settings *Settings `json:"settings"`
}

func (i Container) logger() (entry *log.Entry) {
	return log.WithFields(log.Fields{
		"container": i.Id,
	})
}

func (i Container) Username() (username string) {
	return "sb-" + i.Id
}
