package nat

import (
	"fmt"
	"net"
)

// GatherLocalCandidates enumerates this host's non-loopback interface
// addresses paired with port, for use as LAN fast-path candidates ("host
// candidates" in ICE terms) - tried alongside the tracker-observed
// reflexive (public) address when connecting to a peer.
func GatherLocalCandidates(port int) []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}

	var candidates []string
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		ip4 := ipNet.IP.To4()
		if ip4 == nil {
			continue // skip IPv6 for now; keep the candidate set small and predictable
		}
		candidates = append(candidates, fmt.Sprintf("%s:%d", ip4.String(), port))
	}

	return candidates
}
