package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

func TestObservabilityHTTPFlowAndRestart(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	db, err := store.New(file)
	if err != nil {
		t.Fatal(err)
	}
	srv := New(db, "flow-admin-token-12345678901234567890", "flow-agent-token-12345678901234567890", false)
	now := time.Now().UTC().Add(-time.Minute)
	trace := "abcdef1234567890abcdef1234567890"
	service := "svc:qa:checkout"
	h := srv.Handler()
	call := func(method, path, token string, body any) *httptest.ResponseRecorder {
		var b []byte
		if body != nil {
			b, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-API-Key", token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/api/v1/apm/analysis?service=" + service, "/api/v1/telemetry/logs", "/api/v1/telemetry/metrics", "/api/v1/slos", "/api/v1/apm/rules", "/api/v1/apm/problems"} {
		if w := call("GET", path, srv.AgentToken, nil); w.Code != 401 {
			t.Fatalf("agent read admin %s: %d", path, w.Code)
		}
	}
	spans := []model.Span{
		{TraceID: trace, SpanID: "1111111111111111", ServiceName: "checkout", ServiceNamespace: "qa", Name: "POST /orders", Kind: "server", Status: "error", StartTime: now, EndTime: now.Add(200 * time.Millisecond)},
		{TraceID: trace, SpanID: "2222222222222222", ParentSpanID: "1111111111111111", ServiceName: "checkout", ServiceNamespace: "qa", Name: "SELECT", Kind: "client", StartTime: now, EndTime: now.Add(20 * time.Millisecond), Attributes: map[string]any{"db.system.name": "postgresql", "db.namespace": "orders", "db.operation.name": "SELECT"}},
	}
	if err := db.IngestSpans(spans); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/apm/analysis?" + url.Values{"service": {service}}.Encode()
	w := call("GET", path, srv.AdminToken, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var analysis model.APMAnalysis
	if err := json.Unmarshal(w.Body.Bytes(), &analysis); err != nil {
		t.Fatal(err)
	}
	if len(analysis.DatabaseCalls) != 1 || len(analysis.Failures) != 1 {
		t.Fatalf("missing analysis evidence: %+v", analysis)
	}
	at := fmt.Sprint(now.UnixNano())
	logs := map[string]any{"resourceLogs": []any{map[string]any{"resource": map[string]any{"attributes": []any{map[string]any{"key": "service.name", "value": map[string]any{"stringValue": "checkout"}}, map[string]any{"key": "service.namespace", "value": map[string]any{"stringValue": "qa"}}}}, "scopeLogs": []any{map[string]any{"logRecords": []any{map[string]any{"timeUnixNano": at, "severityNumber": 17, "severityText": "ERROR", "traceId": trace, "spanId": "1111111111111111", "body": map[string]any{"stringValue": "order failed"}}}}}}}}
	if w := call("POST", "/v1/logs", srv.AgentToken, logs); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("GET", "/api/v1/telemetry/logs?"+url.Values{"service": {service}, "trace_id": {trace}}.Encode(), srv.AdminToken, nil)
	var lr struct {
		Logs []model.LogRecord `json:"logs"`
	}
	if json.Unmarshal(w.Body.Bytes(), &lr) != nil || len(lr.Logs) != 1 || lr.Logs[0].Body != "order failed" {
		t.Fatal(w.Body.String())
	}
	policy := model.SLOPolicy{ID: "availability", Name: "Availability", ServiceID: service, Kind: "availability", TargetPercent: 99.5}
	if w := call("POST", "/api/v1/slos", srv.AdminToken, policy); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	rule := model.APMRule{ID: "failures", Name: "Failure rate", ServiceID: service, Metric: "error_rate", Threshold: 50, MinRequests: 1, WindowMinutes: 5, Severity: "critical", Enabled: true}
	if w := call("POST", "/api/v1/apm/rules", srv.AdminToken, rule); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("POST", "/api/v1/apm/rules/evaluate", srv.AdminToken, nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	reopened, err := store.New(file)
	if err != nil {
		t.Fatal(err)
	}
	h = New(reopened, srv.AdminToken, srv.AgentToken, false).Handler()
	if len(reopened.RuleProblems("active")) != 1 {
		t.Fatal("problem missing after restart")
	}
	w = call("GET", "/api/v1/slos", srv.AdminToken, nil)
	var slos struct {
		SLOs []model.SLOEvaluation `json:"slos"`
	}
	json.Unmarshal(w.Body.Bytes(), &slos)
	if len(slos.SLOs) != 1 || slos.SLOs[0].Status != "breached" || slos.SLOs[0].Total != 1 {
		t.Fatal(w.Body.String())
	}
	if w := call(http.MethodGet, "/api/v1/telemetry/logs?limit=201", srv.AdminToken, nil); w.Code != 400 {
		t.Fatal(w.Code)
	}
}
