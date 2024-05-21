package container

import (
	"bufio"
	"errors"
	"github.com/sethvargo/go-password/password"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (c *Container) setupDirectory(directory string) (err error) {
	c.logger().Info("creating sshd directory")
	return os.MkdirAll(directory, os.ModePerm)
}

func (c *Container) setupGroup(group string) (err error) {
	c.logger().Info("creating sshd group")
	output, err := exec.Command("groupadd", "-f", group).Output()
	if err != nil {
		c.logger().Error("sshd group creation error: " + string(output))
	}
	return err
}

func (c *Container) commentOut(original string) (output string) {
	c.logger().Info("commenting out: " + original)
	return "# before serverbench: " + original + "\n"
}

func (c *Container) setupSshdJail(group string, directory string) (err error) {
	c.logger().Info("setting up sshd")
	sshdConfig := "/etc/ssh/sshd_config"
	file, err := os.OpenFile(sshdConfig, os.O_RDWR, 0644)
	if err != nil {
		c.logger().Error("unable to read ftp jail")
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
				c.logger().Info("updating sshd subsystem")
				output += c.commentOut(line)
				line = "Subsystem\tsftp\tinternal-sftp"
				updated = true
			}
			foundSubsystem = true
		} else if strings.HasPrefix(line, "Match Group "+group) {
			foundGroupMatching = true
		}

		output += line + "\n"
	}

	if !foundSubsystem || !foundGroupMatching {
		output += "# appended by serverbench\n"
		updated = true
	}

	if !foundSubsystem {
		c.logger().Info("appending sshd subsystem")
		output += "Subsystem\tsftp\tinternal-sftp\n"
	}

	if !foundGroupMatching {
		c.logger().Info("appending sshd group match rule")
		output += "Match Group " + group + "\n"
		output += "  ForceCommand internal-sftp -d /data\n"
		output += "  ChrootDirectory " + directory + "%u\n"
	}

	if updated {
		c.logger().Info("saving new sshd config")
		overwrite, err := os.Create(sshdConfig)
		if err != nil {
			c.logger().Error("unable to save new sshd config")
			return err
		}
		_, err = overwrite.WriteString(output)
		c.logger().Info("restarting sshd")
		exec.Command("systemctl", "restart", "ssh.service")
	}
	return err
}

func (c *Container) setupMount(home string, username string, group string) (err error) {
	homeData := filepath.Join(home, "data")
	targetData := filepath.Join(c.Path, "data")

	// true data
	err = os.MkdirAll(targetData, os.ModePerm)
	if err != nil {
		c.logger().Error("error while creating true data directory")
		return err
	}
	c.logger().Info("created true data directory")

	// data mount
	err = os.MkdirAll(homeData, os.ModePerm)
	if err != nil {
		c.logger().Error("error while creating data mounting directory")
		return err
	}
	c.logger().Info("created data mounting directory")

	// create mount
	output, err := exec.Command("mount", "--bind", targetData, homeData).Output()
	if err != nil {
		c.logger().Error("error while mounting data: " + string(output))
		return err
	}
	c.logger().Info("mounted data")

	// jail
	err = c.setJail(home)
	if err != nil {
		return err
	}
	err = c.setJail(targetData)
	if err != nil {
		return err
	}
	err = c.setChown(username, targetData, group)
	if err != nil {
		return err
	}

	c.logger().Info("mounted " + home + " to " + c.Path + " for " + username)
	return nil
}

func (c *Container) setChown(username string, path string, group string) (err error) {
	output, err := exec.Command("chown", "-R", username+":"+group, path).Output()
	if err != nil {
		c.logger().Error("error while chowning (target jailing): " + string(output))
	}
	c.logger().Info("target jailed #" + username + " at " + path)
	return err
}

func (c *Container) setJail(path string) (err error) {
	err = os.Chown(path, 0, 0)
	if err != nil {
		c.logger().Error("error while chowning (jailing) " + path + ": " + err.Error())
		return err
	}
	_, err = exec.Command("chmod", "755", path).Output()
	if err != nil {
		c.logger().Error("error while chmodding (jailing) " + path + ": " + err.Error())
	}
	c.logger().Info("jailed " + path)
	return err
}

func (c *Container) ResetPassword() (pswd *string, err error) {
	username := c.Username()
	c.logger().Info("resetting " + username + " password")
	if username == "root" {
		c.logger().Error("protected root password password")
		return nil, errors.New("root unsupported")
	}
	pwd, err := password.Generate(32, 10, 0, false, false)
	if err != nil {
		return nil, err
	}
	pswd = &pwd
	output, err := exec.Command("bash", "-c", "echo \""+username+":"+pwd+"\" | /usr/sbin/chpasswd").Output()
	if err != nil {
		c.logger().Error("password reset for " + username + " error, output was: " + string(output))
		return nil, err
	}
	c.logger().Info("reset " + username + " password: sftp://" + username + ":" + *pswd + "@ip")
	return pswd, nil
}

func (c *Container) createUser() (pswd *string, err error) {
	username := c.Username()
	c.logger().Info("creating user " + username + " (" + c.Path + ")")
	if username == "root" {
		c.logger().Error("protected root creation")
		return nil, errors.New("root unsupported")
	}

	err = c.setupGroup(group)
	if err != nil {
		return nil, err
	}

	err = c.setupDirectory(directory)
	if err != nil {
		return nil, err
	}

	err = c.setupSshdJail(group, directory)
	if err != nil {
		return nil, err
	}

	home := filepath.Join(directory, username)
	output, err := exec.Command("/usr/sbin/useradd", "-m", "-d", home, "-G", group, "--shell", "/bin/false", username).Output()
	if err != nil {
		c.logger().Error("ssh user creation error for #" + username + ", output was: " + string(output))
		return nil, err
	}
	c.logger().Info("created user #" + username)

	err = c.setupMount(home, username, group)
	if err != nil {
		return nil, err
	}

	resetPassword, err := c.ResetPassword()
	if err != nil {
		return nil, err
	}

	c.logger().Info("mounted user " + username + " (" + c.Path + ")")
	return resetPassword, err
}

func (c *Container) removeUser() (err error) {
	username := c.Username()
	c.logger().Info("removing user " + username)
	output, err := exec.Command("/usr/sbin/userdel", "-f", username).Output()
	if err != nil {
		c.logger().Error("unable to delete user " + username + ": " + string(output))
	}
	homeDir := filepath.Join(directory, username)
	c.logger().Info("removing mount: " + homeDir)
	output, err2 := exec.Command("umount", "-l", filepath.Join(homeDir, "data")).Output()
	if err2 != nil {
		c.logger().Error("unable to unmount " + username + ": " + string(output))
		err = err2
	}
	err3 := os.RemoveAll(homeDir)
	if err3 != nil {
		err = err3
		c.logger().Error("unable to delete data: " + homeDir + ": " + err3.Error())
	}
	return err
}
