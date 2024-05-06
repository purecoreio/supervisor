package machine

import (
	"bufio"
	"errors"
	"github.com/sethvargo/go-password/password"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (m Machine) setupDirectory() (err error) {
	m.logger().Info("creating sshd directory")
	return os.MkdirAll(m.Directory, os.ModePerm)
}

func (m Machine) setupGroup() (err error) {
	m.logger().Info("creating sshd group")
	output, err := exec.Command("groupadd", "-f", m.Group).Output()
	if err != nil {
		m.logger().Error("sshd group creation error: " + string(output))
	}
	return err
}

func (m Machine) commentOut(original string) (output string) {
	m.logger().Info("commenting out: " + original)
	return "# before serverbench: " + original + "\n"
}

func (m Machine) setupSshdJail() (err error) {
	m.logger().Info("setting up sshd")
	sshdConfig := "/etc/ssh/sshd_config"
	file, err := os.OpenFile(sshdConfig, os.O_RDWR, 0644)
	if err != nil {
		m.logger().Error("unable to read ftp jail")
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	output := ""

	updated := false
	foundGroupMatching := false
	foundSubsystem := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Subsystem") {
			if !strings.HasSuffix(line, "internal-sftp") {
				m.logger().Info("updating sshd subsystem")
				output += m.commentOut(line)
				line = "Subsystem\tsftp\tinternal-sftp"
				updated = true
			}
			foundSubsystem = true
		} else if strings.HasPrefix(line, "Match Group "+m.Group) {
			foundGroupMatching = true
		}

		output += line + "\n"
	}

	if !foundSubsystem || !foundGroupMatching {
		output += "# appended by serverbench\n"
		updated = true
	}

	if !foundSubsystem {
		m.logger().Info("appending sshd subsystem")
		output += "Subsystem\tsftp\tinternal-sftp\n"
	}

	if !foundGroupMatching {
		m.logger().Info("appending sshd group match rule")
		output += "Match Group " + m.Group + "\n"
		output += "  ForceCommand internal-sftp -d /data\n"
		output += "  ChrootDirectory " + m.Directory + "%u\n"
	}

	if updated {
		m.logger().Info("saving new sshd config")
		overwrite, err := os.Create(sshdConfig)
		if err != nil {
			m.logger().Error("unable to save new sshd config")
			return err
		}
		_, err = overwrite.WriteString(output)
		m.logger().Info("restarting sshd")
		exec.Command("systemctl", "restart", "ssh.service")
	}
	return err
}

func (m Machine) setupMount(home string, username string, target string) (err error) {
	homeData := filepath.Join(home, "data")
	targetData := filepath.Join(target, "data")

	// true data
	err = os.MkdirAll(targetData, os.ModePerm)
	if err != nil {
		m.logger().Error("error while creating true data directory")
		return err
	}
	m.logger().Info("created true data directory")

	// data mount
	err = os.MkdirAll(homeData, os.ModePerm)
	if err != nil {
		m.logger().Error("error while creating data mounting directory")
		return err
	}
	m.logger().Info("created data mounting directory")

	// create mount
	output, err := exec.Command("mount", "--bind", targetData, homeData).Output()
	if err != nil {
		m.logger().Error("error while mounting data: " + string(output))
		return err
	}
	m.logger().Info("mounted data")

	// jail
	err = m.setJail(home)
	if err != nil {
		return err
	}
	err = m.setJail(targetData)
	if err != nil {
		return err
	}
	err = m.setChown(username, targetData)
	if err != nil {
		return err
	}

	m.logger().Info("mounted " + home + " to " + target + " for " + username)
	return nil
}

func (m Machine) setChown(username string, path string) (err error) {
	output, err := exec.Command("chown", "-R", username+":"+m.Group, path).Output()
	if err != nil {
		m.logger().Error("error while chowning (target jailing): " + string(output))
	}
	m.logger().Info("target jailed #" + username + " at " + path)
	return err
}

func (m Machine) setJail(path string) (err error) {
	err = os.Chown(path, 0, 0)
	if err != nil {
		m.logger().Error("error while chowning (jailing) " + path + ": " + err.Error())
		return err
	}
	_, err = exec.Command("chmod", "755", path).Output()
	if err != nil {
		m.logger().Error("error while chmodding (jailing) " + path + ": " + err.Error())
	}
	m.logger().Info("jailed " + path)
	return err
}

func (m Machine) resetPassword(username string) (pswd *string, err error) {
	m.logger().Info("resetting " + username + " password")
	if username == "root" {
		m.logger().Error("protected root password password")
		return nil, errors.New("root unsupported")
	}
	pwd, err := password.Generate(32, 10, 0, false, false)
	if err != nil {
		return nil, err
	}
	pswd = &pwd
	output, err := exec.Command("bash", "-c", "echo \""+username+":"+pwd+"\" | /usr/sbin/chpasswd").Output()
	if err != nil {
		m.logger().Error("password reset for " + username + " error, output was: " + string(output))
		return nil, err
	}
	m.logger().Info("reset " + username + " password: sftp://" + username + ":" + *pswd + "@ip")
	return pswd, nil
}

func (m Machine) createUser(username string, target string) (pswd *string, err error) {
	m.logger().Info("creating user " + username + " (" + target + ")")
	if username == "root" {
		m.logger().Error("`protected root creation")
		return nil, errors.New("root unsupported")
	}

	err = m.setupGroup()
	if err != nil {
		return nil, err
	}

	err = m.setupDirectory()
	if err != nil {
		return nil, err
	}

	err = m.setupSshdJail()
	if err != nil {
		return nil, err
	}

	home := filepath.Join(m.Directory, username)
	output, err := exec.Command("/usr/sbin/useradd", "-m", "-d", home, "-G", m.Group, "--shell", "/bin/false", username).Output()
	if err != nil {
		m.logger().Error("ssh user creation error for #" + username + ", output was: " + string(output))
		return nil, err
	}
	m.logger().Info("created user #" + username)

	err = m.setupMount(home, username, target)
	if err != nil {
		return nil, err
	}

	resetPassword, err := m.resetPassword(username)
	if err != nil {
		return nil, err
	}

	m.logger().Info("created user " + username + " (" + target + ")")
	return resetPassword, err
}

func (m Machine) removeUser(username string) (err error) {
	m.logger().Info("removing user " + username)
	output, err := exec.Command("/usr/sbin/userdel", "-f", username).Output()
	if err != nil {
		m.logger().Error("unable to delete user " + username + ": " + string(output))
	}
	homeDir := filepath.Join(m.Directory, username)
	m.logger().Info("removing mount: " + homeDir)
	output, err2 := exec.Command("umount", "-l", filepath.Join(homeDir, "data")).Output()
	if err2 != nil {
		m.logger().Error("unable to unmount " + username + ": " + string(output))
		err = err2
	}
	err3 := os.RemoveAll(homeDir)
	if err3 != nil {
		err = err3
		m.logger().Error("unable to delete data: " + homeDir + ": " + err3.Error())
	}
	return err
}
