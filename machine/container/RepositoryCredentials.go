package container

import (
	"errors"
	"regexp"
)

type RepositoryCredentials struct {
	Repository Repository `json:"repository"`
	Username   string     `json:"username"`
	Token      string     `json:"token"`
}

func (r *RepositoryCredentials) CheckUsername() (err error) {
	re, err := regexp.Compile("(?i)" + `^[a-z\d](?:[a-z\\d]|-(?=[a-z\d])){0,38}$`)
	if err != nil {
		return err
	}
	match := re.MatchString(r.Username)
	if match == false {
		err = errors.New("invalid git username")
	}
	return err
}

func (r *RepositoryCredentials) CheckToken() (err error) {
	re, err := regexp.Compile(`^ghs_[a-zA-Z0-9]{36}$`)
	if err != nil {
		return err
	}
	match := re.MatchString(r.Token)
	if match == false {
		err = errors.New("invalid git token")
	}
	return err
}
