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
	Firewall FirewallPolicy `json:"whitelist"`
	Rules    []Rule         `json:"rules"`
	Ip       Ip             `json:"ip"`
}

func (p *Port) CreateRules(usedInterface string, chainName string) (err error) {
	usedIpParsed := net.ParseIP(p.Ip.Ip)
	if usedIpParsed == nil {
		return errors.New("invalid source ip")
	}
	var protos = []string{"tcp", "udp"}
	var actionInList string
	var actionOutsideList string
	const drop = "DROP"
	const accept = "ACCEPT"
	if p.Firewall == Whitelist {
		actionInList = accept
		actionOutsideList = drop
	} else {
		actionInList = drop
		actionOutsideList = accept
	}
	for _, rule := range p.Rules {
		resolvedIps, err := rule.GetIps()
		if err == nil {
			for _, ip := range resolvedIps {
				var utility string
				if ip.To4() != nil {
					// IPV4
					utility = "iptables"
				} else {
					// IPV6
					utility = "ip6tables"
				}
				for _, proto := range protos {
					exec.Command(utility, "-A", chainName, "-p", usedInterface, "-s", ip.String(), "-d", p.Ip.Ip, "-p", proto, "--dport", strconv.Itoa(p.Port), "-j", actionInList)
				}
			}
		}
	}
	var utility string
	for _, proto := range protos {
		if usedIpParsed.To4() != nil {
			// IPV4
			utility = "iptables"
		} else {
			// IPV6
			utility = "ip6tables"
		}
		exec.Command(utility, "-A", chainName, "-p", usedInterface, "-d", p.Ip.Ip, "-p", proto, "--dport", strconv.Itoa(p.Port), "-j", actionOutsideList)
	}
	return nil
}
