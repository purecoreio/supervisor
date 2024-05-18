package listener

type Level struct {
	Status   bool `json:"status"`
	Logs     bool `json:"logs"`
	Progress bool `json:"progress"`
}
