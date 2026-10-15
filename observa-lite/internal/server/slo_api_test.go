package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
)

func TestSLOAPIAuthEvaluationPersistenceAndDelete(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	st, err := store.New(file)
	if err != nil {
		t.Fatal(err)
	}
	srv := New(st, "admin-test-token-12345678901234567890", "agent-test-token-12345678901234567890", false)
	h := srv.Handler()
	request := func(method, path, token string, body any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("X-API-Key", token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	p := model.SLOPolicy{ID: "api", Name: "API availability", ServiceID: "svc::api", Kind: "availability", TargetPercent: 99}
	for _, method := range []string{"GET", "POST", "DELETE"} {
		path := "/api/v1/slos"
		if method == "DELETE" {
			path += "/api"
		}
		if w := request(method, path, srv.AgentToken, p); w.Code != 401 {
			t.Fatalf("agent access %s: %d", method, w.Code)
		}
	}
	if w := request("POST", "/api/v1/slos", srv.AdminToken, p); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	// Reopen to prove API-created config is persisted.
	reopened, err := store.New(file)
	if err != nil || len(reopened.SLOPolicies()) != 1 {
		t.Fatalf("restart: %v", err)
	}
	now := time.Now().UTC().Add(-time.Minute)
	span := model.Span{TraceID: "11111111111111111111111111111111", SpanID: "1111111111111111", ServiceName: "api", Name: "GET /", Kind: "server", Status: "error", StartTime: now, EndTime: now.Add(time.Millisecond)}
	if err := st.IngestSpans([]model.Span{span}); err != nil {
		t.Fatal(err)
	}
	w := request("GET", "/api/v1/slos", srv.AdminToken, nil)
	var response struct {
		SLOs []model.SLOEvaluation `json:"slos"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(response.SLOs) != 1 || response.SLOs[0].Total != 1 || response.SLOs[0].Status != "breached" {
		t.Fatalf("evaluation %d: %s", w.Code, w.Body.String())
	}
	for _, target := range []float64{0, -1, 101} {
		v := p
		v.TargetPercent = target
		if w := request("POST", "/api/v1/slos", srv.AdminToken, v); w.Code != 400 {
			t.Fatalf("target %v: %d", target, w.Code)
		}
	}
	if w := request("GET", "/api/v1/slos?from=invalid", srv.AdminToken, nil); w.Code != 400 {
		t.Fatal("invalid window accepted")
	}
	if w := request("DELETE", "/api/v1/slos/missing", srv.AdminToken, nil); w.Code != 404 {
		t.Fatalf("missing: %d", w.Code)
	}
	if w := request("DELETE", "/api/v1/slos/api", srv.AdminToken, nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	after, err := store.New(file)
	if err != nil || len(after.SLOPolicies()) != 0 {
		t.Fatal("delete not persisted")
	}
}

func TestSLOAPIPersistenceFailureIsServerError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "state.json")
	st, _ := store.New(file)
	p := model.SLOPolicy{ID: "api", Name: "API", ServiceID: "svc::api", Kind: "availability", TargetPercent: 99}
	if err := st.PutSLOPolicy(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(file, 0700); err != nil {
		t.Fatal(err)
	}
	srv := New(st, "admin-test-token-12345678901234567890", "agent-test-token-12345678901234567890", false)
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		path := "/api/v1/slos"
		if method == http.MethodDelete {
			path += "/api"
		}
		p.Name = "Changed"
		b, _ := json.Marshal(p)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("X-API-Key", srv.AdminToken)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, r)
		if w.Code != 500 {
			t.Fatalf("persist error %s: %d %s", method, w.Code, w.Body.String())
		}
		if policies := st.SLOPolicies(); len(policies) != 1 || policies[0].Name != "API" {
			t.Fatal("failed mutation changed memory")
		}
	}
}
