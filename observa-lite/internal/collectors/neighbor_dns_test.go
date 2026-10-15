package collectors

import (
	"context"
	"errors"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"testing"
)

func TestNeighborDNSPTRAndFallback(t *testing.T) {
	rows := []model.DeviceNeighbor{{RemoteIP: "10.0.0.1", RemoteName: "LLDP original"}, {RemoteIP: "10.0.0.1"}, {RemoteIP: "10.0.0.2", RemoteName: "No PTR"}, {RemoteIP: "bad"}}
	calls := 0
	resolveNeighborDNS(context.Background(), rows, func(ctx context.Context, ip string) ([]string, error) {
		calls++
		if ip == "10.0.0.2" {
			return nil, errors.New("NXDOMAIN")
		}
		return []string{"SWITCH.EXAMPLE.INTERNAL."}, nil
	})
	if calls != 2 || rows[0].DNSName != "switch.example.internal" || rows[1].DNSName != rows[0].DNSName || rows[0].RemoteName != "LLDP original" || rows[2].DNSName != "" || rows[2].RemoteName != "No PTR" {
		t.Fatalf("calls=%d rows=%+v", calls, rows)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resolveNeighborDNS(ctx, rows, func(context.Context, string) ([]string, error) { t.Fatal("lookup after cancellation"); return nil, nil })
}
