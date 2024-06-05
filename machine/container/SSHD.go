package container

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/sethvargo/go-password/password"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
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

func (c *Container) ResetKeys() (publicKey string, err error) {
	c.logger().Info("resetting keys")
	usr, err := user.Lookup(c.Username())
	if err != nil {
		c.logger().Error("error while looking up user")
		return publicKey, err
	}

	sshDir := filepath.Join(usr.HomeDir, ".ssh")

	// Create the .ssh directory
	err = os.MkdirAll(sshDir, 0700)
	if err != nil {
		c.logger().Error("error while creating ssh directory")
		return publicKey, err
	}

	// Define the authorized_keys file path
	authKeysFile := filepath.Join(sshDir, "authorized_keys")

	// Create the authorized_keys file if it doesn't exist
	_, err = os.OpenFile(authKeysFile, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		c.logger().Error("error while creating authorized_keys file")
		return publicKey, err
	}

	// Change the ownership of the .ssh directory and its contents
	uid := usr.Uid
	gid := usr.Gid
	uidInt, err := strconv.Atoi(uid)
	if err != nil {
		c.logger().Error("error while converting uid to int")
		return publicKey, err
	}
	gidInt, err := strconv.Atoi(gid)
	if err != nil {
		c.logger().Error("error while converting gid to int")
		return publicKey, err
	}

	err = os.Chown(sshDir, uidInt, gidInt)
	if err != nil {
		c.logger().Error("error while chowning authorized_keys")
		return publicKey, err
	}

	err = exec.Command("sh", "-c", `yes y | ssh-keygen -t ed25519 -C "<id>" -f `+path.Join(sshDir, "id_ed25519")+` -N ""`).Run()
	if err != nil {
		c.logger().Error("error while creating ssh key")
		return publicKey, err
	}
	err = os.Chown(authKeysFile, uidInt, gidInt)
	if err != nil {
		c.logger().Error("error while chowning authorized_keys")
		return publicKey, err
	}
	c.logger().Info("reset keys")
	return c.GetPublicKey()
}

func (c *Container) GetPublicKey() (publicKey string, err error) {
	c.logger().Info("reading public key")
	pubKeyBytes, err := os.ReadFile(path.Join(directory, c.Username(), ".ssh", "id_ed25519"))
	if err != nil {
		c.logger().Error("error while looking up public key")
		return "", err
	}
	c.logger().Info("read key")
	return string(pubKeyBytes), nil
}

func (c *Container) AddAuthorizedKey(authorizedKey string) (err error) {
	c.logger().Info("adding authorized key")
	err = c.checkKey(authorizedKey)
	if err != nil {
		return err
	}
	authKeysFile := c.getAuthorizedKeysFile()
	existingKeys, err := c.ListAuthorizedKeys()
	if err != nil {
		return err
	}
	for _, key := range existingKeys {
		if key == authorizedKey {
			// key already exists, ignore
			return nil
		}
	}
	file, err := os.OpenFile(authKeysFile, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open authorized_keys file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(authorizedKey + "\n")
	if err != nil {
		return fmt.Errorf("failed to write to authorized_keys file: %w", err)
	}
	c.logger().Info("added authorized key")
	return nil
}

func (c *Container) RemoveAuthorizedKey(authorizedKey string) (err error) {
	c.logger().Info("removing authorized key")
	err = c.checkKey(authorizedKey)
	if err != nil {
		return err
	}
	authKeysFile := c.getAuthorizedKeysFile()
	existingKeys, err := c.ListAuthorizedKeys()
	if err != nil {
		return err
	}
	found := false
	var output []string
	for _, key := range existingKeys {
		if key != authorizedKey {
			output = append(output, key)
		} else {
			found = true
		}
	}
	if !found {
		err = errors.New("authorized key not found")
	}
	err = os.WriteFile(authKeysFile, []byte(strings.Join(output, "\n")), 0600)
	if err != nil {
		return fmt.Errorf("failed to write to authorized_keys file: %w", err)
	}
	c.logger().Info("removed authorized key")
	return nil
}

func (c *Container) ListAuthorizedKeys() (authorizedKeys []string, err error) {
	authKeysFile := c.getAuthorizedKeysFile()
	file, err := os.Open(authKeysFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open authorized_keys file: %w", err)
	}
	defer file.Close()

	var keys []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		keys = append(keys, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading authorized_keys file: %w", err)
	}

	return keys, nil
}

func (c *Container) getAuthorizedKeysFile() string {
	return path.Join(directory, c.Username(), ".ssh", "authorized_keys")
}

func (c *Container) getPrivateKeyFile() string {
	return path.Join(directory, c.Username(), ".ssh", "id_ed25519")
}

func (c *Container) checkKey(key string) (err error) {
	if len(key) > 8192 {
		err = errors.New("key too long")
		return err
	}
	sshKeyRegex := regexp.MustCompile(`^ssh-(rsa|ed25519|dsa|ecdsa)\s+[A-Za-z0-9+/]+[=]{0,2}\s*[^\s@]+(?:@\S+)?$`)
	matches := sshKeyRegex.MatchString(key)
	if matches == false {
		err = errors.New("invalid key")
	}
	return err
}

func (c *Container) userExists() (exists bool, err error) {
	lookup, err := user.Lookup(c.Username())
	if err != nil {
		if errors.Is(err, user.UnknownUserError(c.Username())) {
			return false, nil
		}
		return false, err
	}
	return lookup != nil, nil
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

	_, err = c.ResetKeys()
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
