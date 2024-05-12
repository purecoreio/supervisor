package container

type HostingTemplate struct {
	Id        string                    `json:"id"`
	Image     HostingImage              `json:"image"`
	Name      *string                   `json:"name"`
	Variables []HostingTemplateVariable `json:"variables"`
}
