package main

type Template struct {
	id     string
	image  string
	memory uint64
	cores  float32
	size   uint64 // the fs size
	mount  string // the mounted path, usually /data
	expiry int64  // the amount of seconds that will take for the container to become expired when deleted, completely deleting it from the system
}
