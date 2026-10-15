package server

import (
	"encoding/json"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestDeviceConnectionsRequireEvidence(t *testing.T) {
	st, _ := store.New(filepath.Join(t.TempDir(), "state.json"))
	now := time.Now()
	for _, h := range []model.Discovery{{HostID: "app", Hostname: "Application host", Addresses: []string{"10.0.0.20"}, CapturedAt: now}, {HostID: "unrelated", Hostname: "Same subnet", Addresses: []string{"10.0.0.21"}, CapturedAt: now}} {
		if e := st.Ingest(h); e != nil {
			t.Fatal(e)
		}
	}
	profile := model.Integration{ID: "switch", Name: "Core switch", Kind: "snmp", Target: "10.0.0.1", SNMPVersion: "2c", TemplateID: "generic-snmp", Enabled: true}
	if e := st.PutIntegrationWithSecret(profile, "encrypted-fixture"); e != nil {
		t.Fatal(e)
	}
	result := model.IntegrationResult{At: now, SuccessAt: now, Status: "ok", Resources: []model.IntegrationResource{{ID: "10.0.0.1", Name: "Core switch", Kind: "network.device", Attributes: map[string]string{"address": "10.0.0.1"}, Interfaces: []model.DeviceInterface{{Index: "1", Name: "Gi1"}}, Neighbors: []model.DeviceNeighbor{{LocalPort: "Gi1", LocalIfIndex: "1", RemoteIP: "10.0.0.20", RemoteName: "Application host", RemotePort: "eth0", Protocol: "LLDP"}}}}}
	if e := st.SaveIntegrationResult(profile, result); e != nil {
		t.Fatal(e)
	}
	if e := st.IngestSpans([]model.Span{{TraceID: "11111111111111111111111111111111", SpanID: "2222222222222222", ServiceName: "Checkout", Name: "GET /", Kind: "server", Status: "ok", StartTime: now.Add(-time.Second), EndTime: now, ResourceAttributes: map[string]any{"host.id": "app"}}}); e != nil {
		t.Fatal(e)
	}
	s := New(st, "admin-topology-test-token-12345", "agent-topology-test-token-12345", true)
	r := httptest.NewRequest("GET", "/api/v1/topology?view=devices", nil)
	r.Header.Set("X-API-Key", s.AdminToken)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	var graph model.Map
	if e := json.Unmarshal(w.Body.Bytes(), &graph); e != nil {
		t.Fatal(e)
	}
	hasLLDP, hasAPM := false, false
	for _, edge := range graph.Edges {
		if edge.To == "host:unrelated" {
			t.Fatal("invented subnet link")
		}
		if edge.Kind == "lldp" && edge.To == "host:app" {
			hasLLDP = true
		}
		if edge.Kind == "runs-service" && edge.From == "host:app" && edge.To == "svc::Checkout" {
			hasAPM = true
		}
	}
	if !hasLLDP || !hasAPM {
		t.Fatalf("relations missing: %+v", graph.Edges)
	}
}
