package event

import (
	"encoding/json"
	"github.com/docker/docker/api/types"
)

type StatusUpdate struct {
	Running    bool   `json:"running"`
	Restarting bool   `json:"restarting"`
	Paused     bool   `json:"paused"`
	Status     string `json:"status"`
}

func (u *StatusUpdate) FromContainerState(state *types.ContainerState) *StatusUpdate {
	u.Running = state.Running
	u.Restarting = state.Restarting
	u.Paused = state.Paused
	u.Status = state.Status
	return u
}

func (u *StatusUpdate) Encode() (string, error) {
	marshal, err := json.Marshal(u)
	if err != nil {
		return "", err
	}
	return string(marshal), err
}
