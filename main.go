package main

import (
	"supervisor/machine"
)

func main() {
	self := machine.Machine{
		Group:     "serverbench",
		Directory: "/etc/serverbench/supervisor/containers/",
		Id:        "<example-machine>",
	}

	err := self.Init("test", 0)
	if err != nil {
		return
	}
}
