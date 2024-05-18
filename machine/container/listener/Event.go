package listener

type Type string

const (
	Log      Type = "log"
	Status        = "status"
	Progress      = "progress"
)

type Event struct {
	Listener  string `json:"listener"`
	Type      Type   `json:"type"`
	Container string `json:"container"`
	Content   string `json:"content"`
}
