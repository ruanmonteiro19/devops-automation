package store

import (
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"path/filepath"
	"testing"
	"time"
)

func TestInventorySettingsSurviveCollectionAndRestart(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	s, _ := New(file)
	d := model.Discovery{HostID: "host", Hostname: "original", CapturedAt: time.Now(), Processes: []model.Process{{PID: 8, Name: "java", Service: "orders", Runtime: "java"}, {PID: 9, Name: "bash", Service: "bash"}, {PID: 10, Name: "(sd-pam)", Service: "(sd-pam)"}}}
	if err := s.Ingest(d); err != nil {
		t.Fatal(err)
	}
	name, group, alias, pid := "Payments host", "Production", "Payments JVM", 8
	if err := s.EditInventory("host", InventoryEdit{Name: &name, Group: &group}); err != nil {
		t.Fatal(err)
	}
	if err := s.EditInventory("host", InventoryEdit{PID: &pid, ProcessName: &alias, OriginalName: &d.Processes[0].Name, OriginalService: &d.Processes[0].Service}); err != nil {
		t.Fatal(err)
	}
	d.Processes[0].PID = 12
	if err := s.Ingest(d); err != nil {
		t.Fatal(err)
	}
	s, err := New(file)
	if err != nil {
		t.Fatal(err)
	}
	v := s.Snapshot()
	h := v.Hosts[0]
	if h.Name != name || h.HostGroup != group || h.Processes[0].DisplayName != alias {
		t.Fatalf("metadata lost: %+v", h)
	}
	if len(v.ProcessMap.Nodes) != 1 || v.ProcessMap.Nodes[0].Name != alias {
		t.Fatalf("system processes leaked: %+v", v.ProcessMap)
	}
	if err := s.EditInventory("host", InventoryEdit{Reset: true}); err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().Hosts[0].Name != "original" {
		t.Fatal("reset did not restore observation")
	}
}
func TestInventoryPersistenceFailureRollsBack(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	s.data.Hosts["h"] = model.Host{ID: "h", Name: "Original"}
	s.file = "/dev/null/state"
	name := "Changed"
	if s.EditInventory("h", InventoryEdit{Name: &name}) == nil {
		t.Fatal("expected failure")
	}
	if s.Snapshot().Hosts[0].Name != "Original" {
		t.Fatal("failed save changed memory")
	}
}
