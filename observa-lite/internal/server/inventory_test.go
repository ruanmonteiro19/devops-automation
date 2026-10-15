package server

import (
	"bytes"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestInventoryEditRequiresAdmin(t *testing.T) {
	st, _ := store.New(filepath.Join(t.TempDir(), "db.json"))
	_ = st.Ingest(model.Discovery{HostID: "h", Hostname: "Original"})
	srv := New(st, "admin-test-token-12345678901234567890", "agent-test-token-12345678901234567890", false)
	for _, tc := range []struct {
		token  string
		status int
	}{{"", 401}, {srv.AgentToken, 401}, {srv.AdminToken, 200}} {
		r := httptest.NewRequest("PATCH", "/api/v1/hosts/h", bytes.NewBufferString(`{"name":"Payments","host_group":"New group"}`))
		r.Header.Set("X-API-Key", tc.token)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	}
	h := st.Snapshot().Hosts[0]
	if h.Name != "Payments" || h.HostGroup != "New group" {
		t.Fatalf("not saved: %+v", h)
	}
}
