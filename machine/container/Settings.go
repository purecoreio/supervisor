package container

type Settings struct {
	Ports   map[int]int       `json:"ports"`
	Envs    map[string]string `json:"envs"`
	Volume  string            `json:"volume"`
	Path    string            `json:"path"`
	Ip      string            `json:"ip"`
	Memory  int               `json:"memory"`
	Storage *int              `json:"storage"`
}
