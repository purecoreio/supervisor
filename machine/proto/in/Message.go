package in

type Message struct {
	Rid     string  `json:"rid"`
	Realm   string  `json:"realm"`
	Command string  `json:"command"`
	Target  *string `json:"target"`
	Data    *string `json:"data"`
}
