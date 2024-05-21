package event

type Type string

const (
	Log      Type = "log"
	Status        = "status"
	Progress      = "progress"
	Load          = "load"
)
