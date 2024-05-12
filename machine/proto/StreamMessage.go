package proto

type StreamMessage struct {
	Rid    string `json:"rid"`
	Stream bool   `json:"stream"`
	Start  bool   `json:"start"`
	End    bool   `json:"end"`
	Data   string `json:"data"`
}
