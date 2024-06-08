package container

import (
	"errors"
	"os/exec"
	"strconv"
)

const pre = "sb"

func (c *Container) ChainName() string {
	return pre + "-" + c.Id
}

func (c *Container) flushChain() (err error) {
	return exec.Command("iptables", "-F", c.ChainName()).Run()
}

func (c *Container) deleteChain() (err error) {
	c.logger().Info("deleting chain")
	err = c.flushChain()
	err = exec.Command("iptables", "-X", c.ChainName()).Run()
	if err != nil {
		c.logger().Error("error while deleting chain: ", err)
		return err
	}
	c.logger().Info("deleted chain")
	return nil
}

func (c *Container) ApplyRules() (err error) {
	c.logger().Info("creating chain")
	err = c.createChainIfNeeded()
	if err != nil {
		c.logger().Error("error while creating chain: ", err)
		return err
	}
	c.logger().Info("flushing rules")
	err = c.flushChain()
	if err != nil {
		c.logger().Error("error while flushing chain: ", err)
		return err
	}
	c.logger().Info("applying new rules")
	for _, port := range c.Ports {
		err = port.CreateRules(c.ChainName())
		if err != nil {
			c.logger().Error("error while creating rules for "+strconv.Itoa(port.Port)+": ", err)
			return err
		}
	}
	c.logger().Info("created chain")
	return nil
}

func (c *Container) createChainIfNeeded() (err error) {
	cmd := exec.Command("iptables", "-L", c.ChainName())
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
		err = exec.Command("iptables", "-N", c.ChainName()).Run()
		if err != nil {
			return err
		}
		err = exec.Command("iptables", "-I", "FORWARD", "-j", c.ChainName()).Run()
		if err != nil {
			return err
		}
	}
	return nil
}
