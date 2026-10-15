package server

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkspaceMCPPersistenceAndWorkflow(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	db, e := store.New(file)
	if e != nil {
		t.Fatal(e)
	}
	s := New(db, "admin-token-test-01234567890123456789", "agent-token-test-01234567890123456789", false)
	h := s.Handler()
	call := func(token, method, path string, p any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(p)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("X-API-Key", token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	o := model.WorkspaceObject{ID: "book", Name: "Investigação", Kind: "notebook", Cells: []model.NotebookCell{{ID: "q", Kind: "query", Content: "fetch hosts"}}}
	rpc := func(apply bool) *httptest.ResponseRecorder {
		return call("admin-token-test-01234567890123456789", "POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "configure_workspace", "arguments": map[string]any{"object": o, "apply": apply}}})
	}
	if w := call("agent-token-test-01234567890123456789", "POST", "/api/v1/workspace", o); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := rpc(false); w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(`"isError":true`)) {
		t.Fatal(w.Body.String())
	}
	if len(db.WorkspaceObjects("")) != 0 {
		t.Fatal("preview wrote state")
	}
	if w := rpc(true); w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(`"isError":true`)) {
		t.Fatal(w.Body.String())
	}
	reloaded, e := store.New(file)
	if e != nil || len(reloaded.WorkspaceObjects("notebook")) != 1 {
		t.Fatalf("persistence %v", e)
	}
	flow := model.WorkspaceObject{ID: "flow", Name: "Conditional", Kind: "workflow", Steps: []model.WorkflowStep{{ID: "q", Name: "Empty", Kind: "query", Query: "fetch hosts"}, {ID: "condition", Kind: "condition", MinRows: 1}, {ID: "never", Kind: "database_check", TargetID: "missing"}}}
	if e := db.PutWorkspace(flow); e != nil {
		t.Fatal(e)
	}
	run, e := s.startOperation(context.Background(), flow.ID)
	if e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		runs := db.OperationRuns(flow.ID)
		if len(runs) > 0 && runs[0].Status != "running" {
			if runs[0].ID != run.ID || runs[0].Status != "success" || runs[0].Result["skipped_remaining"] != true {
				t.Fatalf("%+v", runs)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("workflow did not finish")
}
func TestSoftwareImportProductionOnly(t *testing.T) {
	p, e := ParseSoftwareDocument(json.RawMessage(`{"packages":{"":{"name":"app"},"node_modules/lodash":{"version":"4.17.20"},"node_modules/test":{"version":"1","dev":true}}}`))
	if e != nil || len(p) != 1 || p[0].Name != "lodash" {
		t.Fatalf("%+v %v", p, e)
	}
	p, e = ParseSoftwareDocument(json.RawMessage(`{"bomFormat":"CycloneDX","components":[{"purl":"pkg:npm/%40scope/name@1.2.3","version":"1.2.3"}]}`))
	if e != nil || len(p) != 1 || p[0].Name != "@scope/name" {
		t.Fatalf("%+v %v", p, e)
	}
}
