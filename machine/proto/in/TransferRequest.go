package in

type TransferRequest struct {
	Out      bool    `json:"out"`
	Address  string  `json:"address"`
	User     string  `json:"user"`
	Path     string  `json:"path"`
	Password *string `json:"password,omitempty"`
	Mirror   bool    `json:"mirror"`
	Port     int     `json:"port"`
}
