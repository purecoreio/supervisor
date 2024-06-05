package event

import (
	"encoding/json"
)

type ProgressUpdate struct {
	Id          string  `json:"id"`
	Description string  `json:"description"`
	Started     bool    `json:"started"`
	Finished    bool    `json:"finished"`
	Errored     bool    `json:"errored"`
	Progress    int     `json:"progress"`
	Type        string  `json:"type"`
	HeadSha     *string `json:"headSha"`
}

func (p *ProgressUpdate) Encode() (content string, err error) {
	contentBytes, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	content = string(contentBytes)
	return content, err
}
