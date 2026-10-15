package server

import (
	"bytes"
	"encoding/json"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRuleAPIAuthLifecycleAndErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "db.json")
	st, _ := store.New(file)
	srv := New(st, "admin-test-token-12345678901234567890", "agent-test-token-12345678901234567890", false)
	h := srv.Handler()
	call := func(method, path, token string, body any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("X-API-Key", token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	rule := model.APMRule{ID: "errors", Name: "Errors", ServiceID: "svc::api", Metric: "error_rate", Threshold: 50, MinRequests: 1, WindowMinutes: 5, Severity: "critical", Enabled: true}
	for _, route := range []struct{ method, path string }{{"GET", "/api/v1/apm/rules"}, {"POST", "/api/v1/apm/rules"}, {"DELETE", "/api/v1/apm/rules/errors"}, {"POST", "/api/v1/apm/rules/evaluate"}, {"GET", "/api/v1/apm/problems"}} {
		if w := call(route.method, route.path, srv.AgentToken, rule); w.Code != 401 {
			t.Fatal("agent access", route, w.Code)
		}
	}
	if w := call("POST", "/api/v1/apm/rules", srv.AdminToken, rule); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	now := time.Now().UTC().Add(-time.Minute)
	st.IngestSpans([]model.Span{{TraceID: "11111111111111111111111111111111", SpanID: "1111111111111111", ServiceName: "api", Kind: "server", Status: "error", StartTime: now, EndTime: now.Add(time.Millisecond)}})
	for i := 0; i < 2; i++ {
		if w := call("POST", "/api/v1/apm/rules/evaluate", srv.AdminToken, nil); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := call("GET", "/api/v1/apm/problems?status=active", srv.AdminToken, nil)
	var out struct {
		Problems  []model.RuleProblem `json:"problems"`
		CausalRCA bool                `json:"causal_rca"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != 200 || len(out.Problems) != 1 || out.CausalRCA {
		t.Fatal("problems", w.Body.String())
	}
	reopened, err := store.New(file)
	if err != nil || len(reopened.RuleProblems("active")) != 1 || len(reopened.APMRules()) != 1 {
		t.Fatal("restart", err)
	}
	invalid := rule
	invalid.MinRequests = 0
	if w := call("POST", "/api/v1/apm/rules", srv.AdminToken, invalid); w.Code != 400 {
		t.Fatal("validation", w.Code)
	}
	if w := call("GET", "/api/v1/apm/problems?status=unknown", srv.AdminToken, nil); w.Code != 400 {
		t.Fatal("invalid status")
	}
	if w := call("DELETE", "/api/v1/apm/rules/missing", srv.AdminToken, nil); w.Code != 404 {
		t.Fatal("missing", w.Code)
	}
	// Failed storage preserves both policy and incident state.
	os.Remove(file)
	os.Mkdir(file, 0700)
	rule.Enabled = false
	if w := call("POST", "/api/v1/apm/rules", srv.AdminToken, rule); w.Code != 500 {
		t.Fatal("persistence", w.Code)
	}
	if !st.APMRules()[0].Enabled || len(st.RuleProblems("active")) != 1 {
		t.Fatal("failed mutation modified state")
	}
	os.Remove(file)
	if w := call("DELETE", "/api/v1/apm/rules/errors", srv.AdminToken, nil); w.Code != 200 {
		t.Fatal("delete", w.Code)
	}
	if len(st.RuleProblems("active")) != 0 || len(st.RuleProblems("resolved")) != 1 {
		t.Fatal("delete did not close incident")
	}
}
