package ip

import (
	"errors"
	"net"
	"os/exec"
	"strconv"
)

type FirewallPolicy string

const (
	Blacklist FirewallPolicy = "blacklist"
	Whitelist                = "whitelist"
)

type Port struct {
	Port     int            `json:"port"`
	Firewall FirewallPolicy `json:"firewall"`
	Rules    []Rule         `json:"rules"`
	Ip       Ip             `json:"ip"`
}

func (p *Port) CreateRules(chainName string) (err error) {
	if p.Port == 22 {
		return errors.New("can't modify ssh port")
	}
	usedIpParsed := net.ParseIP(p.Ip.Ip)
	if usedIpParsed == nil {
		return errors.New("invalid source ip")
	}
	var utility string
	if usedIpParsed.To4() != nil {
		// IPV4
		utility = "iptables"
	} else {
		// IPV6
		utility = "ip6tables"
	}
	var protos = []string{"tcp", "udp"}
	var actionInList string
	var actionOutsideList string
	const drop = "DROP"
	const accept = "ACCEPT"
	if p.Firewall == Whitelist {
		actionInList = accept
		actionOutsideList = drop
	} else if p.Firewall == Blacklist {
		actionInList = drop
		actionOutsideList = accept
	} else {
		err = errors.New("unknown firewall policy")
		return err
	}
	for _, rule := range p.Rules {
		resolvedIps, err := rule.GetIps()
		if err == nil {
			for _, ip := range resolvedIps {
				for _, proto := range protos {
					err = exec.Command(utility, "-A", chainName, "-p", p.Ip.Adapter, "-s", ip.String(), "-d", p.Ip.Ip, "-p", proto, "--dport", strconv.Itoa(p.Port), "-j", actionInList).Run()
					if err != nil {
						return err
					}
				}
			}
		}
	}
	for _, proto := range protos {
		err = exec.Command(utility, "-A", chainName, "-p", p.Ip.Adapter, "-d", p.Ip.Ip, "-p", proto, "--dport", strconv.Itoa(p.Port), "-j", actionOutsideList).Run()
		if err != nil {
			return err
		}
	}
	return nil
}
