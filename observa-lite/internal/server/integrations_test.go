package server

import (
	"context"
	"encoding/json"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIntegrationAPIAndTopology(t *testing.T) {
	st, _ := store.New(filepath.Join(t.TempDir(), "state.json"))
	s := New(st, "admin-token-integration-test-12345", "agent-token-integration-test-12345", true)
	call := func(method, path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-API-Key", token)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/api/v1/integrations", "", "agent-token-integration-test-12345"); w.Code != 401 {
		t.Fatal("agent accessed admin integrations")
	}
	body := `{"id":"core","name":"Core switch","kind":"snmp_exporter","enabled":true,"endpoint":"http://localhost:9116","target":"10.0.0.1","module":"if_mib","auth":"core_v3"}`
	if w := call("POST", "/api/v1/integrations", body, "admin-token-integration-test-12345"); w.Code != 200 {
		t.Fatalf("save %d %s", w.Code, w.Body.String())
	}
	p, _ := st.Integration("core")
	result := model.IntegrationResult{At: time.Now(), SuccessAt: time.Now(), Status: "ok", Resources: []model.IntegrationResource{{ID: "10.0.0.1", Name: "Core", Kind: "network.device", Interfaces: []model.DeviceInterface{{Index: "1", Name: "Gi1", Metrics: map[string]float64{"ifOperStatus": 1}}}}}}
	if e := st.SaveIntegrationResult(p, result); e != nil {
		t.Fatal(e)
	}
	w := call("GET", "/api/v1/topology?view=devices", "", "admin-token-integration-test-12345")
	var graph model.Map
	if e := json.Unmarshal(w.Body.Bytes(), &graph); e != nil || len(graph.Nodes) != 3 || len(graph.Edges) != 2 {
		t.Fatalf("topology %s %v", w.Body.String(), e)
	}
	for _, edge := range graph.Edges {
		if edge.Kind != "contains" {
			t.Fatal("invented traffic relation")
		}
	}
	if w := call("GET", "/api/v1/topology?view=unknown", "", "admin-token-integration-test-12345"); w.Code != 400 {
		t.Fatal("unvalidated view")
	}
	if w := call("DELETE", "/api/v1/integrations/core", "", "admin-token-integration-test-12345"); w.Code != 200 {
		t.Fatal("delete")
	}
}
func TestFailedCollectKeepsLastSuccessAndRejectsConcurrentSync(t *testing.T) {
	st, _ := store.New(filepath.Join(t.TempDir(), "state.json"))
	s := New(st, "admin-token-integration-test-12345", "agent-token-integration-test-12345", true)
	p := model.Integration{ID: "aws", Name: "Cloud", Kind: "aws", Region: "us-east-1", CredentialPrefix: "OBSERVA_ABSENT", Enabled: true}
	_ = st.PutIntegration(p)
	now := time.Now().Add(-time.Minute)
	_ = st.SaveIntegrationResult(p, model.IntegrationResult{At: now, SuccessAt: now, Status: "ok", Resources: []model.IntegrationResource{{ID: "old", Name: "Last inventory"}}})
	result, e := s.collectProfile(context.Background(), p)
	if e != nil || result.Status != "error" || len(result.Resources) != 1 || !result.SuccessAt.Equal(now) {
		t.Fatalf("failure semantics %+v %v", result, e)
	}
	release, e := s.reserveProfile(p)
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	if _, e := s.collectProfile(context.Background(), p); e != errIntegrationBusy {
		t.Fatal("duplicate sync allowed")
	}
	if s.Store.IntegrationResults()[p.ID].Status != "error" {
		t.Fatal("error not persisted")
	}
}
func TestAsyncDeviceCollectionEndpoint(t *testing.T) {
	exporter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ifOperStatus{ifIndex=\"1\",ifName=\"Gi1\"} 1\n"))
	}))
	defer exporter.Close()
	st, _ := store.New(filepath.Join(t.TempDir(), "state.json"))
	s := New(st, "admin-token-integration-test-12345", "agent-token-integration-test-12345", true)
	p := model.Integration{ID: "device", Name: "Core", Kind: "snmp_exporter", Endpoint: exporter.URL, Target: "10.0.0.1", Module: "if_mib", Auth: "auth", Enabled: true}
	_ = st.PutIntegration(p)
	r := httptest.NewRequest("POST", "/api/v1/integrations/device/collect", nil)
	r.Header.Set("X-API-Key", "admin-token-integration-test-12345")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 202 {
		t.Fatalf("async %d", w.Code)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if result, ok := st.IntegrationResults()[p.ID]; ok {
			if result.Status != "ok" || len(result.Resources) != 1 {
				t.Fatalf("result %+v", result)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("async collection did not persist")
}

func TestTopologyKubernetesAndAIRequireObservedAttributes(t *testing.T) {
	st, _ := store.New(filepath.Join(t.TempDir(), "state.json"))
	s := New(st, "admin-token-integration-test-12345", "agent-token-integration-test-12345", false)
	now := time.Now().UTC().Add(-time.Minute)
	span := model.Span{TraceID: strings.Repeat("1", 32), SpanID: strings.Repeat("2", 16), Name: "LLM call", ServiceName: "api", Kind: "client", Status: "ok", StartTime: now, EndTime: now.Add(time.Millisecond), ResourceAttributes: map[string]any{"k8s.cluster.name": "cluster", "k8s.namespace.name": "ns", "k8s.pod.uid": "pod-uid"}, Attributes: map[string]any{"gen_ai.provider.name": "provider"}}
	if e := st.IngestSpans([]model.Span{span}); e != nil {
		t.Fatal(e)
	}
	for view, count := range map[string]int{"kubernetes": 3, "ai": 1} {
		r := httptest.NewRequest("GET", "/api/v1/topology?view="+view, nil)
		r.Header.Set("X-API-Key", s.AdminToken)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		var graph model.Map
		if e := json.Unmarshal(w.Body.Bytes(), &graph); e != nil || len(graph.Nodes) != count {
			t.Fatalf("%s topology %s %v", view, w.Body.String(), e)
		}
		for _, n := range graph.Nodes {
			if n.Name == "" {
				t.Fatal("empty observed identity")
			}
		}
	}
}
