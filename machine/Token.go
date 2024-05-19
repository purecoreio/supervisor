package machine

import (
	"errors"
	"os"
)

func (m *Machine) GetToken() (token *string, err error) {
	m.logger().Info("reading token")
	path, err := m.getTokenPath()
	if err != nil {
		return nil, err
	}
	tokenBytes, err := os.ReadFile(*path)
	if err != nil {
		m.logger().Error("error reading token")
		return nil, err
	}
	tokenVal := string(tokenBytes)
	if len(tokenVal) <= 0 {
		return nil, errors.New("the token is empty")
	}
	token = &tokenVal
	m.logger().Info("token read")
	return token, err
}

func (m *Machine) getTokenPath() (path *string, err error) {
	// ensure the token path exists
	m.logger().Info("accessing token store")
	accPath := "/etc/serverbench"
	err = os.MkdirAll(accPath, os.ModePerm)
	if err != nil {
		m.logger().Error("error while accessing/creating token store")
		return nil, err
	}
	accPath += "/supervisor.token"
	file, err := os.OpenFile(accPath, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		m.logger().Error("error touching token")
		return nil, err
	}
	err = file.Close()
	if err != nil {
		m.logger().Error("error detaching (touch) token")
		return nil, err
	}
	m.logger().Info("exited token store")
	path = &accPath
	return path, err
}

func (m *Machine) UpdateToken(token string) (err error) {
	if len(token) <= 0 {
		return errors.New("the token is empty")
	}
	path, err := m.getTokenPath()
	if err != nil {
		return err
	}
	m.logger().Info("updating token")
	file, err := os.OpenFile(*path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		m.logger().Error("error while modifying token store")
		return err
	}
	_, err = file.WriteString(token)
	if err != nil {
		m.logger().Error("error while writing into token store")
		return err
	}
	err = file.Close()
	if err != nil {
		m.logger().Error("error while exiting token store")
		return err
	}
	m.logger().Info("updated token")
	return err
}
