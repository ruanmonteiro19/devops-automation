package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ruanmonteiro19/observa-lite/internal/store"
)

func TestCapabilitiesDoNotAdvertiseVisualStubsAsFeatures(t *testing.T) {
	db, e := store.New(filepath.Join(t.TempDir(), "s.json"))
	if e != nil {
		t.Fatal(e)
	}
	srv := New(db, "admin-test-token-12345678901234567890", "agent-test-token-12345678901234567890", false)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	req.Header.Set("X-API-Key", srv.AdminToken)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Features map[string]bool `json:"features"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	for _, feature := range []string{"grafana_dashboard_import", "zabbix_lld", "deep_apm", "gateway_heartbeat", "mfa", "oidc_sso", "ai_rca", "otlp_grpc"} {
		if out.Features[feature] {
			t.Fatalf("%s is not implemented and must not be advertised as supported", feature)
		}
	}
	for _, feature := range []string{"service_map", "traces", "otlp_http_json", "otlp_http_protobuf", "apm_red", "service_map_api", "process_mapping", "network_ip_flows", "gateway_manual_routes", "template_http_tcp"} {
		if !out.Features[feature] {
			t.Fatalf("%s has a functional basic implementation and was not advertised", feature)
		}
	}
}
