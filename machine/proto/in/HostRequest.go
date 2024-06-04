package in

import "supervisor/machine/container"

type HostRequest struct {
	Container container.Container `json:"container"`
	Token     *string             `json:"token,omitempty"`
}
