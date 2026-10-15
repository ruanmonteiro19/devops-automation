package collectors

import (
	"context"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/ruanmonteiro19/observa-lite/internal/model"
)

// PTR labels enrich observed neighbors; DNS alone never creates physical links.
// Resolve each IP once per collection, with bounded work and a shared time budget.
func resolveNeighborDNS(parent context.Context, neighbors []model.DeviceNeighbor, lookup func(context.Context, string) ([]string, error)) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	names := map[string]string{}
	for i := range neighbors {
		ip := net.ParseIP(neighbors[i].RemoteIP)
		if ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
			continue
		}
		key := ip.String()
		if name, ok := names[key]; ok {
			neighbors[i].DNSName = name
			continue
		}
		if len(names) >= 64 || ctx.Err() != nil {
			continue
		}
		request, done := context.WithTimeout(ctx, 500*time.Millisecond)
		answers, err := lookup(request, key)
		done()
		name := ""
		if err == nil {
			sort.Strings(answers)
			for _, answer := range answers {
				candidate := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(answer), "."))
				if validNeighborDNS(candidate) {
					name = candidate
					break
				}
			}
		}
		names[key] = name
		neighbors[i].DNSName = name
	}
}

func validNeighborDNS(name string) bool {
	if name == "" || len(name) > 253 || net.ParseIP(name) != nil {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}
