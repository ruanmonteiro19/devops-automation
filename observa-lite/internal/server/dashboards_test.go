package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
)

func apiDashboard() model.Dashboard {
	return model.Dashboard{ID: "ops", Title: "Operations", Panels: []model.Panel{{ID: "p1", Title: "Requests", Kind: "apm-stat", SourceID: "observa-apm", Query: `{"service_id":"svc::checkout","metric":"requests"}`}}}
}
func TestDashboardAPIAuthCreateEditSnapshotDelete(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	st, err := store.New(file)
	if err != nil {
		t.Fatal(err)
	}
	srv := New(st, "admin-token-12345678901234567890", "agent-token-12345678901234567890", false)
	handler := srv.Handler()
	call := func(method, path, token string, v any) *httptest.ResponseRecorder {
		var body bytes.Buffer
		if v != nil {
			if err := json.NewEncoder(&body).Encode(v); err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, &body)
		req.Header.Set("X-API-Key", token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}
	for _, token := range []string{"", srv.AgentToken} {
		for _, method := range []string{"POST", "DELETE"} {
			path := "/api/v1/dashboards"
			if method == "DELETE" {
				path += "/ops"
			}
			if rr := call(method, path, token, apiDashboard()); rr.Code != 401 {
				t.Fatalf("dashboard auth failed: %d", rr.Code)
			}
		}
	}
	dashboard := apiDashboard()
	if rr := call("POST", "/api/v1/dashboards", srv.AdminToken, dashboard); rr.Code != 200 {
		t.Fatalf("create failed: %d %s", rr.Code, rr.Body.String())
	}
	dashboard.Title = "Updated"
	dashboard.Panels[0].Kind = "apm-series"
	if rr := call("POST", "/api/v1/dashboards", srv.AdminToken, dashboard); rr.Code != 200 {
		t.Fatalf("edit failed: %d %s", rr.Code, rr.Body.String())
	}
	rr := call("GET", "/api/v1/snapshot", srv.AdminToken, nil)
	var snapshot model.Snapshot
	if err = json.Unmarshal(rr.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Dashboards) != 1 || snapshot.Dashboards[0].Title != "Updated" || snapshot.Dashboards[0].Panels[0].Kind != "apm-series" {
		t.Fatalf("snapshot lost dashboard: %+v", snapshot.Dashboards)
	}
	reload, err := store.New(file)
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := reload.Dashboard("ops")
	if !ok || saved.Title != "Updated" {
		t.Fatal("dashboard edit was not persisted")
	}
	if rr := call("DELETE", "/api/v1/dashboards/ops", srv.AdminToken, nil); rr.Code != 200 {
		t.Fatalf("delete failed: %d %s", rr.Code, rr.Body.String())
	}
	if rr := call("DELETE", "/api/v1/dashboards/ops", srv.AdminToken, nil); rr.Code != 404 {
		t.Fatalf("missing dashboard returned %d", rr.Code)
	}
	bad := apiDashboard()
	bad.Panels[0].Kind = "fake"
	if rr := call("POST", "/api/v1/dashboards", srv.AdminToken, bad); rr.Code != 400 {
		t.Fatalf("invalid dashboard returned %d: %s", rr.Code, rr.Body.String())
	}
	bad = apiDashboard()
	bad.Title = strings.Repeat("x", 257)
	if rr := call("POST", "/api/v1/dashboards", srv.AdminToken, bad); rr.Code != 400 {
		t.Fatalf("long title returned %d", rr.Code)
	}
}
func TestDashboardAPIPersistenceFailure500AndRollback(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	st, err := store.New(file)
	if err != nil {
		t.Fatal(err)
	}
	dashboard := apiDashboard()
	if err = st.PutDashboard(dashboard); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(file+".tmp", 0700); err != nil {
		t.Fatal(err)
	}
	srv := New(st, "admin-token-12345678901234567890", "agent-token-12345678901234567890", false)
	handler := srv.Handler()
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		var body bytes.Buffer
		path := "/api/v1/dashboards"
		if method == http.MethodPost {
			dashboard.Title = "Not committed"
			_ = json.NewEncoder(&body).Encode(dashboard)
		} else {
			path += "/ops"
		}
		req := httptest.NewRequest(method, path, &body)
		req.Header.Set("X-API-Key", srv.AdminToken)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != 500 {
			t.Fatalf("persistence error returned %d: %s", rr.Code, rr.Body.String())
		}
		saved, ok := st.Dashboard("ops")
		if !ok || saved.Title != "Operations" {
			t.Fatal("failed API write changed dashboard")
		}
	}
}
