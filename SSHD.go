package main

import (
	"bufio"
	"errors"
	"github.com/sethvargo/go-password/password"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (m Machine) changeComment(original string) (output string) {
	return "# before purecore's machine supervisor: " + original + "\n"
}

func (m Machine) getGroup() (groupName string) {
	return "purecore-container"
}

func (m Machine) createGroupIfNeeded() (err error) {
	m.Log("creating sshd group "+m.getGroup()+" (if needed)", Debug)
	output, err := exec.Command("groupadd", "-f", m.getGroup()).Output()
	if err != nil {
		m.Log("sshd group creation error, output was: "+string(output), Debug)
	}
	return err
}

func (m Machine) homePath() (path string) {
	return "/etc/purecore/supervisor/containers/"
}

func (m Machine) createHomeContainer() (er error) {
	return os.MkdirAll(m.homePath(), os.ModePerm)
}

func (m Machine) setJail(path string) (err error) {
	err = os.Chown(path, 0, 0)
	if err != nil {
		return err
	}
	_, err = exec.Command("chmod", "755", path).Output()
	return err
}

func (m Machine) setupSSHD() (err error) {

	m.Log("setting up sshd", Debug)
	// create entry point
	err = m.createHomeContainer()
	if err != nil {
		m.Log("error while creating sshd login entry point", Debug)
		return err
	}

	// create purecore group (if needed)
	err = m.createGroupIfNeeded()
	if err != nil {
		m.Log("error while sshd group", Debug)
		return err
	}

	// modify sshd subsystem and add group match
	sshdConfigPath := "/etc/ssh/sshd_config"
	file, err := os.OpenFile(sshdConfigPath, os.O_RDWR, 0644)
	if err != nil {
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
				m.Log("updating sshd subsystem", Debug)
				output += m.changeComment(line)
				line = "Subsystem\tsftp\tinternal-sftp"
				updated = true
			}
			foundSubsystem = true
		} else if strings.HasPrefix(line, "Match Group "+m.getGroup()) {
			foundGroupMatching = true
		}

		output += line + "\n"
	}

	if !foundSubsystem || !foundGroupMatching {
		output += "# appended by purecore's machine supervisor\n"
		updated = true
	}

	if !foundSubsystem {
		m.Log("appending sshd subsystem", Debug)
		output += "Subsystem\tsftp\tinternal-sftp\n"
	}

	if !foundGroupMatching {
		m.Log("appending sshd group match rule", Debug)
		output += "Match Group " + m.getGroup() + "\n"
		output += "  ForceCommand internal-sftp -d /data\n"
		output += "  ChrootDirectory " + m.homePath() + "%u\n"
	}

	if updated {
		m.Log("saving new sshd config", Debug)
		overwrite, err := os.Create(sshdConfigPath)
		if err != nil {
			return err
		}
		_, err = overwrite.WriteString(output)
		m.Log("restarting sshd", Debug)
		exec.Command("systemctl", "restart", "ssh.service")
	}

	return err

}

func (c Container) ResetPassword() (pswd string, err error) {
	c.machine.Log("resetting #"+c.id+"'s password", Debug)
	pswd, err = password.Generate(32, 10, 0, false, false)
	if err != nil {
		return "", err
	}
	if c.id == "root" {
		return "", errors.New("protected a request to change root's password")
	}
	output, err := exec.Command("bash", "-c", "echo \""+c.id+":"+pswd+"\" | /usr/sbin/chpasswd").Output()
	if err != nil {
		c.machine.Log("password reset for #"+c.id+" error, output was: "+string(output), Debug)
		return "", err
	}
	c.machine.Log("user #"+c.id+" can connect using sftp://"+c.id+":"+pswd+"@65.108.83.183", Debug)
	return pswd, nil
}

func (c Container) Chown(path string) (err error) {
	_, err = exec.Command("chown", "-R", c.id+":"+c.machine.getGroup(), path).Output()
	return err
}

func (c Container) CreateUser() (pswd string, err error) {

	// create the user

	homeDir := filepath.Join(c.machine.homePath(), c.id)
	homeDataDir := filepath.Join(homeDir, "data")
	actualDataDir := filepath.Join(c.storage.path, c.id)
	innerActualData := filepath.Join(actualDataDir, "data")

	// creating data folder

	c.machine.Log("creating login entry point link for user #"+c.id, Debug)
	err = os.MkdirAll(innerActualData, os.ModePerm)
	if err != nil {
		c.machine.Log("error while creating login entry point folder user #"+c.id+": "+err.Error(), Debug)
		return "", err
	}

	// create user

	c.machine.Log("creating ssh user #"+c.id, Debug)
	output, err := exec.Command("/usr/sbin/useradd", "-m", "-d", homeDir, "-G", c.machine.getGroup(), "--shell", "/bin/false", c.id).Output()
	if err != nil {
		c.machine.Log("ssh user creation error for #"+c.id+", output was: "+string(output), Debug)
		return "", err
	}

	// create data folder (will be used as a mount point)
	err = os.MkdirAll(homeDataDir, os.ModePerm)

	// create a password

	pswd, err = c.ResetPassword()
	if err != nil {
		c.machine.Log("error while resetting password for user #"+c.id, Debug)
		return "", err
	}

	// mounting ../data folder onto the users home data mount point

	output, err = exec.Command("mount", "--bind", innerActualData, homeDataDir).Output()
	if err != nil {
		c.machine.Log("error while mounting the ref directory for user #"+c.id+": "+string(output), Debug)
		return "", err
	}

	// jail user

	err = c.machine.setJail(homeDir)
	if err != nil {
		c.machine.Log("error while jailing user #"+c.id+": "+err.Error(), Debug)
		return "", err
	}
	err = c.machine.setJail(actualDataDir)
	if err != nil {
		c.machine.Log("error while jailing user #"+c.id+": "+err.Error(), Debug)
		return "", err
	}

	// chown data

	err = c.Chown(innerActualData)
	if err != nil {
		c.machine.Log("error while chowning the login home for the user #"+c.id+": "+err.Error(), Debug)
		return "", err
	}
	return pswd, err
}

func (c Container) RemoveUser() (err error) {
	c.machine.Log("removing ssh user #"+c.id, Debug)
	_, err = exec.Command("/usr/sbin/userdel", "-f", c.id).Output()
	homeDir := filepath.Join(c.machine.homePath(), c.id)
	exec.Command("umount", "-l", filepath.Join(homeDir, "data"))
	return os.RemoveAll(homeDir)
}
