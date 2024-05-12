package proto

type Response struct {
	Rid   string      `json:"rid"`
	Type  string      `json:"type"`
	Data  interface{} `json:"data"`
	Error bool        `json:"error"`
}
