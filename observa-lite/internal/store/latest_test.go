package store

import (
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"path/filepath"
	"testing"
	"time"
)

func TestLatestValuesAndExactIntegerDelta(t *testing.T) {
	s, _ := NewSQL(filepath.Join(t.TempDir(), "state.json"), "sqlite", "")
	now := time.Now()
	makePoint := func(at time.Time, value string, group string) model.MetricPoint {
		return model.MetricPoint{Name: "network.snmp.counter", ServiceName: "Switch", Kind: "sum", Timestamp: at, Data: map[string]any{"asInt": value}, ResourceAttributes: map[string]any{"device.id": "switch", "host.group": group}, Attributes: map[string]any{"interface.index": "1"}}
	}
	first := makePoint(now.Add(-time.Minute), "9007199254740992", "Old group")
	last := makePoint(now, "9007199254740993", "New group")
	if e := s.IngestMetrics([]model.MetricPoint{last, first}); e != nil {
		t.Fatal(e)
	}
	rows := s.LatestData()
	if len(rows) != 1 || rows[0].Value != "9007199254740993" || rows[0].Change != "1" {
		t.Fatalf("latest precision/order/group identity: %+v", rows)
	}
	history := s.LatestHistory(rows[0].ID, now.Add(-2*time.Minute), now.Add(time.Second))
	if len(history) != 2 || history[0].Value != first.Data["asInt"] {
		t.Fatal("history lost exact values")
	}
	if got := s.LatestDataFiltered(func(row LatestItem) bool { return row.Group == "Old group" }); len(got) != 0 {
		t.Fatal("old sample substituted for filtered current sample")
	}
}
