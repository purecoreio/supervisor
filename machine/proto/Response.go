package proto

type Response struct {
	Rid     string  `json:"rid"`
	Content *string `json:"content"`
	Error   bool    `json:"error"`
}
