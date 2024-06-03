package container

import (
	"errors"
	"regexp"
)

type Repository struct {
	Uri string `json:"uri"`
}

func (r *Repository) CheckToken(token string) (err error) {
	re, err := regexp.Compile(`^ghs_[a-zA-Z0-9]{36}$`)
	if err != nil {
		return err
	}
	match := re.MatchString(token)
	if match == false {
		err = errors.New("invalid git token")
	}
	return err
}
