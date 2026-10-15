package store

import (
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkspaceReferencesIsolationAndRecovery(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	db, e := New(file)
	if e != nil {
		t.Fatal(e)
	}
	o := model.WorkspaceObject{ID: "db", Name: "Primary", Kind: "database", Engine: "postgresql", CredentialEnv: "OBSERVA_DB_PRIMARY_DSN"}
	if e = db.PutWorkspace(o); e != nil {
		t.Fatal(e)
	}
	flow := model.WorkspaceObject{ID: "flow", Name: "Check", Kind: "workflow", Steps: []model.WorkflowStep{{ID: "db", Kind: "database_check", TargetID: "db"}}}
	if e = db.PutWorkspace(flow); e != nil {
		t.Fatal(e)
	}
	if e = db.DeleteWorkspace("db"); e == nil {
		t.Fatal("deleted referenced target")
	}
	loaded, _ := db.WorkspaceObject("flow")
	loaded.Steps[0].TargetID = "other"
	again, _ := db.WorkspaceObject("flow")
	if again.Steps[0].TargetID != "db" {
		t.Fatal("read mutated stored object")
	}
	if e = db.SaveOperation(model.OperationRun{ID: "interrupted", ObjectID: "flow", Kind: "workflow", Status: "running", StartedAt: time.Now()}); e != nil {
		t.Fatal(e)
	}
	reopened, e := New(file)
	if e != nil {
		t.Fatal(e)
	}
	runs := reopened.OperationRuns("flow")
	if len(runs) != 1 || runs[0].Status != "error" || runs[0].Error == "" {
		t.Fatalf("%+v", runs)
	}
}
