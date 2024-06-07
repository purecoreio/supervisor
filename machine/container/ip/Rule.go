package ip

import (
	"net"
	"sync"
	"time"
)

type Rule struct {
	SourceIp     *string
	SourceDomain *string
	resolvedIps  []net.IP
	lastUpdate   time.Time
	checkMutex   sync.Mutex
}

func (r *Rule) GetIps() (ips []net.IP, err error) {
	r.checkMutex.Lock()
	defer r.checkMutex.Unlock()
	now := time.Now()
	twoMinutesAgo := now.Add(-2 * time.Minute)
	requiresUpdate := false
	if r.resolvedIps == nil {
		requiresUpdate = true
	} else if r.lastUpdate.IsZero() || r.lastUpdate.Before(twoMinutesAgo) {
		requiresUpdate = true
	}
	if requiresUpdate {
		if r.SourceDomain != nil {
			ips, err = net.LookupIP(*r.SourceDomain)
		} else {
			ips, err = net.LookupIP(*r.SourceIp)
		}
		r.lastUpdate = now
		r.resolvedIps = ips
	}
	ips = r.resolvedIps
	return ips, err
}
