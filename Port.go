package main

const (
	UDP  string = "UDP"
	TCP         = "TCP"
	BOTH        = "*"
)

type PortConfig struct {
	protocol string
}
