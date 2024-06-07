package container

type HostingImage struct {
	Id           string `json:"id"`
	Uri          string `json:"uri"`
	DefaultMount string `json:"defaultMount"`
}
