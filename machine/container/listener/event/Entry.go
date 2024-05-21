package event

type Entry struct {
	Listener  string `json:"listener"`
	Type      Type   `json:"type"`
	Container string `json:"container"`
	Content   string `json:"content"`
}
