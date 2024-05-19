package listener

type Subscriber struct {
	Id         string    `json:"id"`
	Containers *[]string `json:"containers"`
	Level      *Level    `json:"level"`
}
