package machine

import (
	"errors"
	"os/exec"
)

const chainName = "serverbench"

func (m *Machine) flushChain() (err error) {
	return exec.Command("iptables", "-F", chainName).Run()
}

func (m *Machine) createChainIfNeeded() (err error) {
	cmd := exec.Command("iptables", "-L", chainName)
	_, err = cmd.CombinedOutput()
	var missingChain bool
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			missingChain = true
		} else {
			return err
		}
	} else {
		missingChain = false
	}
	if missingChain {
		err = exec.Command("iptables", "-N", chainName).Run()
		if err != nil {
			return err
		}
		err = exec.Command("iptables", "-I", "FORWARD", "-j", chainName).Run()
		if err != nil {
			return err
		}
	}
	return nil
}
