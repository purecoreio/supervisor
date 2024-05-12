package main

import "supervisor/machine"

var m = machine.Machine{
	Group:     "serverbench",
	Directory: "/etc/serverbench/supervisor/containers/",
}

func main() {
	Execute()
}
