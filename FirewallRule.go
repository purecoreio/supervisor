package main

type FirewallRule struct {
	protocol string
	allow    []string
	block    []string
}
