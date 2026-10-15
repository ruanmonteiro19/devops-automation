package server

import (
	"encoding/json"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

func TestAPMAuthenticatedWindowFilteringAndTraceLookup(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	srv := New(st, "admin-test-token-12345678901234567890", "agent-test-token-12345678901234567890", false)
	now := time.Now().UTC().Add(-time.Minute)
	spans := []model.Span{
		{TraceID: "11111111111111111111111111111111", SpanID: "1111111111111111", ServiceName: "api", ServiceNamespace: "a:b", Name: "GET /", Kind: "server", StartTime: now, EndTime: now.Add(time.Millisecond)},
		{TraceID: "11111111111111111111111111111111", SpanID: "2222222222222222", ParentSpanID: "1111111111111111", ServiceName: "db", Name: "query", Kind: "client", Status: "error", StartTime: now, EndTime: now.Add(time.Millisecond)},
	}
	if err := st.IngestSpans(spans); err != nil {
		t.Fatal(err)
	}
	h := srv.Handler()
	get := func(path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("X-API-Key", token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/api/v1/apm/services", "/api/v1/apm/traces", "/api/v1/apm/map", "/api/v1/apm/traces/" + spans[0].TraceID} {
		if w := get(path, srv.AgentToken); w.Code != 401 {
			t.Fatalf("agent can read admin data %s: %d", path, w.Code)
		}
		if w := get(path, srv.AdminToken); w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	for _, query := range []string{"?from=bad", "?to=bad", "?from=2026-01-01T00:00:00Z&to=2026-01-03T00:00:00Z", "?from=2026-01-03T00:00:00Z&to=2026-01-01T00:00:00Z"} {
		if w := get("/api/v1/apm/services"+query, srv.AdminToken); w.Code != 400 {
			t.Fatalf("bad window accepted %s", query)
		}
	}
	var result struct {
		Traces []model.APMTrace `json:"traces"`
	}
	// The api span succeeded: downstream db failure must not mark this service's own span failed.
	w := get("/api/v1/apm/traces?failed=true&service="+url.QueryEscape("svc:a%3Ab:api"), srv.AdminToken)
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Traces) != 0 {
		t.Fatal("failure predicate crossed service boundary")
	}
	w = get("/api/v1/apm/traces?failed=true&service=db", srv.AdminToken)
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result.Traces) != 1 {
		t.Fatal("actual failed downstream trace missing")
	}
	if w := get("/api/v1/apm/traces/not-found", srv.AdminToken); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w := get("/api/v1/apm/traces?limit=201", srv.AdminToken); w.Code != 400 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/traces", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("OTLP ingest unauthenticated: %d", w.Code)
	}
}
