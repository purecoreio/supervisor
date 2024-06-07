package ip

type Ip struct {
	Ip        string `json:"ip"`
	Adapter   string `json:"adapter"`
	Available bool   `json:"available"`
}
