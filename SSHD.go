package main

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
)

func changeComment(original string) (output string) {
	return "# before purecore's machine supervisor: " + original + "\n"
}

func getGroup() (groupName string) {
	return "purecore-container"
}

func createGroupIfNeeded() {
	exec.Command("groupadd", "-f", getGroup())
}

func setupSSHD() (err error) {

	// create purecore group (if needed)
	createGroupIfNeeded()

	// modify sshd subsystem and add group match
	file, err := os.Open("/etc/ssh/sshd_config")
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	output := ""

	foundGroupMatching := false
	foundSubsystem := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Subsystem") && !strings.HasSuffix(line, "internal-sftp") {
			output += changeComment(line)
			line = "Subsystem\tsftp\tinternal-sftp"
			foundSubsystem = true
		} else if strings.HasPrefix(line, "Match Group "+getGroup()) {
			foundGroupMatching = true
		}

		output += line + "\n"
	}

	output += "# appended by purecore's machine supervisor\n"
	if !foundSubsystem {
		output += "Subsystem\tsftp\tinternal-sftp\n"
	}

	if !foundGroupMatching {
		output += "Match Group " + getGroup() + "\n"
		output += "  ForceCommand internal-sftp\n"
		output += "  ChrootDirectory %h\n"
	}

	// TODO check if this appends
	_, err = file.WriteString(output)

	return err

}

func (c Container) CreateUser() {
	exec.Command("useradd", "-m", "-d "+c.storage.path+string(os.PathSeparator)+c.id, "-G "+getGroup(), c.id)
}

func (c Container) RemoveUser() {
	exec.Command("userdel", c.id)
}
