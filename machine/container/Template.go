package container

type Template struct {
	Id        string     `json:"id"`
	Image     string     `json:"image"`
	Variables []Variable `json:"variables"`
}
