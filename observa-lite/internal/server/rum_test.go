package server

import (
	"bytes"
	"encoding/json"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestRUMOriginConsentAuthenticationAndApdex(t *testing.T) {
	st, _ := store.New(filepath.Join(t.TempDir(), "db.json"))
	srv := New(st, "admin-123456789012345678901234", "agent-123456789012345678901234", false)
	handler := srv.Handler()
	call := func(method, path, token, origin string, body any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Origin", origin)
		r.Header.Set("X-API-Key", token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	app := model.RUMApplication{ID: "web", Name: "Web", Origins: []string{"https://shop.test"}, Enabled: true, RequireConsent: true, SampleRate: 1, Masking: "all", ApdexT: 1000}
	if w := call("POST", "/api/v1/rum/apps", srv.AgentToken, "", app); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := call("POST", "/api/v1/rum/apps", srv.AdminToken, "", app); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("GET", "/rum/config/web", "", "https://evil.test", nil); w.Code != 403 || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("bad CORS", w.Code)
	}
	if w := call("GET", "/rum/config/web", "", "https://shop.test", nil); w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(srv.AdminToken)) {
		t.Fatal("invalid config", w.Code)
	}
	events := []model.RUMEvent{}
	for i, d := range []float64{800, 2500, 5000} {
		events = append(events, model.RUMEvent{ID: []string{"a", "b", "c"}[i], SessionID: "session", PageID: []string{"pagea", "pageb", "pagec"}[i], At: time.Now().UTC(), Kind: "page_view", URL: "https://shop.test/", Duration: d})
	}
	if w := call("POST", "/rum/beacon/web", "", "https://shop.test", map[string]any{"events": events}); w.Code != 403 {
		t.Fatal("missing consent accepted", w.Code)
	}
	if w := call("POST", "/rum/beacon/web", "", "https://shop.test", map[string]any{"consent": true, "events": events}); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("GET", "/api/v1/rum/analysis?app_id=web", srv.AgentToken, "", nil); w.Code != 401 {
		t.Fatal("agent read allowed")
	}
	w := call("GET", "/api/v1/rum/analysis?app_id=web", srv.AdminToken, "", nil)
	var out struct {
		Apdex    float64 `json:"apdex"`
		Pages    int     `json:"pages"`
		Sessions int     `json:"sessions_total"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Apdex != 0.5 || out.Pages != 3 || out.Sessions != 1 {
		t.Fatalf("incorrect analysis: %s", w.Body.String())
	}
	if w := call("DELETE", "/api/v1/rum/apps/web/sessions/session", srv.AdminToken, "", nil); w.Code != 200 {
		t.Fatal("purge", w.Code)
	}
	w = call("GET", "/api/v1/rum/analysis?app_id=web", srv.AdminToken, "", nil)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"apdex":null`)) {
		t.Fatal("empty apdex must be null", w.Body.String())
	}
}
