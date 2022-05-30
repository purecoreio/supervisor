package main

import (
	"bufio"
	"errors"
	"fmt"
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
	err := os.MkdirAll(m.homePath(), os.ModePerm)
	if err != nil {
		return err
	}
	return m.setJail(m.homePath())
}

func (m Machine) setJail(path string) (err error) {
	err = os.Chown(path, 0, 0)
	if err != nil {
		return err
	}
	return os.Chmod(path, 755)
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
		output += "  ForceCommand internal-sftp\n"
		output += "  ChrootDirectory /etc/purecore/supervisor/containers/%u\n"
	}

	if updated {
		m.Log("saving new sshd config", Debug)
		overwrite, err := os.Create(sshdConfigPath)
		if err != nil {
			return err
		}
		_, err = overwrite.WriteString(output)
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
	fmt.Println("echo \"" + c.id + ":" + pswd + "\" | /usr/sbin/chpasswd")
	output, err := exec.Command("bash", "-c", "echo \""+c.id+":"+pswd+"\" | /usr/sbin/chpasswd").Output()
	if err != nil {
		c.machine.Log("password reset for #"+c.id+" error, output was: "+string(output), Debug)
		return "", err
	}
	fmt.Println("sftp://" + c.id + ":" + pswd + "@65.108.83.183")
	return pswd, nil
}

func (c Container) CreateUser() (pswd string, err error) {
	// create the user
	c.machine.Log("creating ssh user #"+c.id, Debug)
	output, err := exec.Command("/usr/sbin/useradd", "-m", "-d", c.storage.path+string(os.PathSeparator)+c.id, "-G", c.machine.getGroup(), c.id).Output()
	if err != nil {
		c.machine.Log("ssh user creation error for #"+c.id+", output was: "+string(output), Debug)
		return "", err
	}
	// create a password
	pswd, err = c.ResetPassword()
	c.machine.Log("creating chroot subdirectory for user #"+c.id, Debug)
	output, err = exec.Command("install", "-d", "-m", "0755", "-o", c.id, "-g", c.machine.getGroup(), c.storage.path+string(os.PathSeparator)+c.id+string(os.PathSeparator)+"data").Output()
	if err != nil {
		c.machine.Log("error while creating chroot subdirectory for user #"+c.id+", output was: "+string(output), Debug)
		return "", err
	}
	// link the sshd login entry point to the user's home
	c.machine.Log("creating login entry point link for user #"+c.id, Debug)
	symlink := filepath.Join(c.machine.homePath(), c.id)
	err = os.Symlink(c.storage.path+string(os.PathSeparator)+c.id+string(os.PathSeparator), symlink)
	if err != nil {
		c.machine.Log("error while creating login entry point link for user #"+c.id+": "+err.Error(), Debug)
		return "", err
	}
	// jailing parent directory
	err = c.machine.setJail(c.storage.path + string(os.PathSeparator))
	if err != nil {
		c.machine.Log("error while jailing original parent directory for user #"+c.id+": "+err.Error(), Debug)
		return "", err
	}
	return pswd, err
}

func (c Container) RemoveUser() {
	c.machine.Log("removing ssh user #"+c.id, Debug)
	exec.Command("/usr/sbin/userdel", c.id)
}
