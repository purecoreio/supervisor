package machine

import "os"

func (m Machine) getToken() (token *string, err error) {
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
	*token = string(tokenBytes)
	m.logger().Info("token read")
	return token, err
}

func (m Machine) getTokenPath() (path *string, err error) {
	// ensure the token path exists
	m.logger().Info("accessing token store")
	*path = "/etc/serverbench/supervisor/token"
	err = os.MkdirAll(*path, os.ModePerm)
	if err != nil {
		m.logger().Error("error while accessing/creating token store")
		return nil, err
	}
	m.logger().Info("exited token store")
	return path, err
}

func (m Machine) updateToken(token string) (err error) {
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
