package store

import (
	"encoding/json"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSQLiteMigrationIncrementalRestartAndUsers(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	legacy, e := New(file)
	if e != nil {
		t.Fatal(e)
	}
	if e = legacy.Ingest(model.Discovery{HostID: "host", Hostname: "Host"}); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(file)
	db, e := NewSQL(file, "sqlite", "")
	if e != nil {
		t.Fatal(e)
	}
	if e = db.BootstrapUsers(); e != nil {
		t.Fatal(e)
	}
	logs := []model.LogRecord{{Timestamp: time.Now().UTC(), ServiceName: "api", Body: strings.Repeat("Repeated body ", 200)}}
	if e = db.IngestLogs(logs); e != nil {
		t.Fatal(e)
	}
	var payload []byte
	if e = db.sql.db.QueryRow("SELECT payload FROM vetra_records WHERE kind='logs'").Scan(&payload); e != nil {
		t.Fatal(e)
	}
	if len(payload) > 500 {
		t.Fatalf("payload not compressed: %d", len(payload))
	}
	rows, _ := db.sql.db.Query("SELECT COUNT(*) FROM vetra_records WHERE kind='logs'")
	var count int
	rows.Next()
	rows.Scan(&count)
	rows.Close()
	if count != 1 {
		t.Fatal(count)
	}
	var wrote int
	db.sql.db.QueryRow("SELECT total_changes()").Scan(&wrote)
	if e = db.IngestLogs(logs); e != nil {
		t.Fatal(e)
	}
	var after int
	db.sql.db.QueryRow("SELECT total_changes()").Scan(&after)
	if wrote != after {
		t.Fatalf("deduplicated batch rewrote database: %d→%d", wrote, after)
	}
	token, _, _, e := db.Login("admin", "12345")
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	db, e = NewSQL(file, "sqlite", "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, _, ok := db.Session(token); !ok {
		t.Fatal("session did not persist")
	}
	if len(db.data.Logs) != 1 || len(db.Snapshot().Hosts) != 1 {
		t.Fatal("migration/restart lost data")
	}
	original, _ := os.ReadFile(file)
	if string(before) != string(original) {
		t.Fatal("original JSON modified")
	}
	b, _ := json.Marshal(db.Users())
	if strings.Contains(string(b), "password_hash") || strings.Contains(string(b), "12345") {
		t.Fatal("password leaked")
	}
	// Empty maps are initialized after reopening a SQL store.
	if e = db.PutWorkspace(model.WorkspaceObject{ID: "book", Name: "Book", Kind: "notebook", Cells: []model.NotebookCell{{ID: "c", Kind: "query", Content: "fetch hosts"}}}); e != nil {
		t.Fatal(e)
	}
}

func TestCompressedStorageFootprint(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "state.json")
	jsonDB, _ := New(file)
	logs := make([]model.LogRecord, 2000)
	for i := range logs {
		logs[i] = model.LogRecord{Timestamp: time.Now().UTC().Add(time.Duration(i) * time.Microsecond), ServiceName: "payments", Body: strings.Repeat("Repetitive structured operation data ", 70), Attributes: map[string]any{"record": i}}
	}
	if e := jsonDB.IngestLogs(logs); e != nil {
		t.Fatal(e)
	}
	info, _ := os.Stat(file)
	sqlite, e := NewSQL(file, "sqlite", "")
	if e != nil {
		t.Fatal(e)
	}
	defer sqlite.Close()
	var compressed int64
	if e = sqlite.sql.db.QueryRow(`SELECT SUM(length(payload)) FROM vetra_records`).Scan(&compressed); e != nil {
		t.Fatal(e)
	}
	if compressed >= info.Size()/2 {
		t.Fatalf("insufficient compression: %d vs %d", compressed, info.Size())
	}
	var busy, frames, checkpointed int
	sqlite.sql.db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &frames, &checkpointed)
	var disk int64
	for _, suffix := range []string{".sqlite", ".sqlite-wal", ".sqlite-shm"} {
		if info, e := os.Stat(file + suffix); e == nil {
			disk += info.Size()
		}
	}
	t.Logf("SQLite physical files after checkpoint=%d bytes; JSON=%d bytes", disk, info.Size())
	t.Logf("JSON snapshot=%d bytes; compressed SQL record payload=%d bytes; ratio=%.3f (excludes database indexes/WAL)", info.Size(), compressed, float64(compressed)/float64(info.Size()))
}

func TestRetentionRemovesSQLRowsWithoutNewTraffic(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	db, e := NewSQL(file, "sqlite", "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = db.IngestLogs([]model.LogRecord{{Timestamp: time.Now(), ServiceName: "api", Body: "Will expire"}}); e != nil {
		t.Fatal(e)
	}
	if e = db.PruneExpired(time.Now().Add(25 * time.Hour)); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = db.sql.db.QueryRow(`SELECT COUNT(*) FROM vetra_records WHERE kind='logs'`).Scan(&count); e != nil || count != 0 {
		t.Fatal("expired SQL rows retained", count, e)
	}
	if len(db.data.Logs) != 0 {
		t.Fatal("expired hot records retained")
	}
}
