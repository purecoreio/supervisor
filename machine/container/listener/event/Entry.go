package event

type Entry struct {
	Listeners []string `json:"listeners"`
	Type      Type     `json:"type"`
	Container string   `json:"container"`
	Content   string   `json:"content"`
}
