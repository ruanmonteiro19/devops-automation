package store

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"io"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type SQLStorage struct {
	db           *sql.DB
	engine       string
	path         string
	fingerprints map[string][32]byte
	metadata     [32]byte
}
type sqlRecord struct {
	kind, id       string
	at             int64
	service, trace string
	raw            []byte
}

var gzipWriters = sync.Pool{New: func() any { w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed); return w }}

func packed(raw []byte) []byte {
	if len(raw) < 512 {
		return append([]byte{0}, raw...)
	}
	var b bytes.Buffer
	b.WriteByte(1)
	z := gzipWriters.Get().(*gzip.Writer)
	z.Reset(&b)
	z.Write(raw)
	z.Close()
	z.Reset(io.Discard)
	gzipWriters.Put(z)
	if b.Len() >= len(raw)+1 {
		return append([]byte{0}, raw...)
	}
	return b.Bytes()
}
func unpacked(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("empty stored payload")
	}
	if b[0] == 0 {
		return b[1:], nil
	}
	if b[0] != 1 {
		return nil, fmt.Errorf("unsupported payload encoding")
	}
	z, e := gzip.NewReader(bytes.NewReader(b[1:]))
	if e != nil {
		return nil, e
	}
	defer z.Close()
	raw, e := io.ReadAll(io.LimitReader(z, 100<<20+1))
	if len(raw) > 100<<20 {
		return nil, fmt.Errorf("stored payload too large")
	}
	return raw, e
}
func (s *Store) sqlRecords() ([]sqlRecord, error) {
	out := []sqlRecord{}
	var err error
	add := func(kind, id string, at time.Time, service, trace string, v any) {
		if err != nil {
			return
		}
		var raw []byte
		raw, err = json.Marshal(v)
		if id == "" {
			h := sha256.Sum256(raw)
			id = hex.EncodeToString(h[:])
		}
		out = append(out, sqlRecord{kind, id, at.UnixNano(), service, trace, raw})
	}
	for _, v := range s.data.Hosts {
		add("hosts", v.ID, v.LastSeen, v.Name, "", v)
	}
	for id, v := range s.data.IntegrationResults {
		add("integration_results", id, v.At, "", "", v)
	}
	for _, v := range s.data.Spans {
		add("spans", spanKey(v), v.StartTime, v.ServiceName, v.TraceID, v)
	}
	for _, v := range s.data.Logs {
		add("logs", "", v.Timestamp, v.ServiceName, v.TraceID, v)
	}
	for _, v := range s.data.MetricPoints {
		add("metrics", "", v.Timestamp, v.ServiceName, "", v)
	}
	for _, v := range s.data.RUMEvents {
		add("rum", v.AppID+":"+v.SessionID+":"+v.ID, v.At, v.AppID, "", v)
	}
	for _, v := range s.data.RUMReplay {
		add("replay", v.AppID+":"+v.SessionID+":"+v.ID, v.At, v.AppID, "", v)
	}
	return out, err
}

// NewSQL migrates an existing JSON snapshot once, preserving the original file.
// Queries retain the existing bounded hot window; this is not a distributed lakehouse.
func NewSQL(legacyFile, engine, dsn string) (*Store, error) {
	if engine != "sqlite" && engine != "postgres" {
		return nil, fmt.Errorf("storage must be json, sqlite or postgres")
	}
	if engine == "sqlite" {
		if dsn == "" {
			dsn = legacyFile + ".sqlite"
		}
		if e := os.MkdirAll(filepath.Dir(dsn), 0700); e != nil {
			return nil, e
		}
		f, e := os.OpenFile(dsn, os.O_CREATE|os.O_RDWR, 0600)
		if e != nil {
			return nil, e
		}
		f.Close()
	}
	var db *sql.DB
	var e error
	if engine == "postgres" {
		if dsn == "" {
			return nil, fmt.Errorf("VETRA_STORAGE_DSN é obrigatório para PostgreSQL")
		}
		cfg, err := pgx.ParseConfig(dsn)
		if err != nil {
			return nil, fmt.Errorf("configuração PostgreSQL inválida")
		}
		cfg.ConnectTimeout = 5 * time.Second
		cfg.RuntimeParams["statement_timeout"] = "15000"
		db = stdlib.OpenDB(*cfg)
	} else {
		db, e = sql.Open("sqlite", dsn)
		if e != nil {
			return nil, e
		}
	}
	db.SetMaxOpenConns(1)
	closeError := func(err error) (*Store, error) { db.Close(); return nil, err }
	if engine == "sqlite" {
		for _, q := range []string{"PRAGMA busy_timeout=5000", "PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA wal_autocheckpoint=1000"} {
			if _, e = db.Exec(q); e != nil {
				return closeError(e)
			}
		}
	}
	for _, q := range []string{`CREATE TABLE IF NOT EXISTS vetra_metadata (key TEXT PRIMARY KEY,payload BYTEA NOT NULL)`, `CREATE TABLE IF NOT EXISTS vetra_records (kind TEXT NOT NULL,id TEXT NOT NULL,at BIGINT NOT NULL,service TEXT NOT NULL,trace TEXT NOT NULL,payload BYTEA NOT NULL,PRIMARY KEY(kind,id))`, `CREATE INDEX IF NOT EXISTS vetra_records_time ON vetra_records(kind,at)`, `CREATE INDEX IF NOT EXISTS vetra_records_trace ON vetra_records(trace)`, `CREATE INDEX IF NOT EXISTS vetra_records_service ON vetra_records(kind,service,at)`} {
		if _, e = db.Exec(q); e != nil {
			return closeError(e)
		}
	}
	var payload []byte
	e = db.QueryRow(`SELECT payload FROM vetra_metadata WHERE key='state'`).Scan(&payload)
	loadedHashes := map[string][32]byte{}
	var s *Store
	if e == sql.ErrNoRows {
		if engine == "postgres" {
			if _, err := os.Stat(legacyFile + ".sqlite"); err == nil {
				db.Close()
				return nil, fmt.Errorf("SQLite existente: exporte o estado e forneça o JSON de migração em --data")
			}
		}
		s, e = New(legacyFile)
		if e != nil {
			return closeError(e)
		}
	} else if e != nil {
		return closeError(e)
	} else {
		raw, err := unpacked(payload)
		if err != nil {
			return closeError(err)
		}
		s = &Store{file: legacyFile}
		if e = json.Unmarshal(raw, &s.data); e != nil {
			return closeError(e)
		}
		rows, err := db.Query(`SELECT kind,id,payload FROM vetra_records ORDER BY at,id`)
		if err != nil {
			return closeError(err)
		}
		for rows.Next() {
			var kind, id string
			var b []byte
			if err = rows.Scan(&kind, &id, &b); err != nil {
				break
			}
			b, err = unpacked(b)
			if err != nil {
				break
			}
			loadedHashes[kind+":"+id] = sha256.Sum256(b)
			switch kind {
			case "hosts":
				var v model.Host
				err = json.Unmarshal(b, &v)
				if s.data.Hosts == nil {
					s.data.Hosts = map[string]model.Host{}
				}
				s.data.Hosts[id] = v
			case "integration_results":
				var v model.IntegrationResult
				err = json.Unmarshal(b, &v)
				if s.data.IntegrationResults == nil {
					s.data.IntegrationResults = map[string]model.IntegrationResult{}
				}
				s.data.IntegrationResults[id] = v
			case "spans":
				err = appendRecord(b, &s.data.Spans)
			case "logs":
				err = appendRecord(b, &s.data.Logs)
			case "metrics":
				err = appendRecord(b, &s.data.MetricPoints)
			case "rum":
				err = appendRecord(b, &s.data.RUMEvents)
			case "replay":
				err = appendRecord(b, &s.data.RUMReplay)
			}
			if err != nil {
				break
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return closeError(err)
		}
	}
	s.initialize()
	s.sql = &SQLStorage{path: dsn, db: db, engine: engine, fingerprints: map[string][32]byte{}}
	s.sql.fingerprints = loadedHashes
	if e = s.persistSQL(); e != nil {
		return closeError(e)
	}
	return s, nil
}
func appendRecord[T any](b []byte, list *[]T) error {
	var v T
	if e := json.Unmarshal(b, &v); e != nil {
		return e
	}
	*list = append(*list, v)
	return nil
}
func (s *Store) persistSQL() error {
	metadata := s.data
	metadata.Hosts = nil
	metadata.IntegrationResults = nil
	metadata.Spans = nil
	metadata.Logs = nil
	metadata.MetricPoints = nil
	metadata.RUMEvents = nil
	metadata.RUMReplay = nil
	raw, e := json.Marshal(metadata)
	if e != nil {
		return e
	}
	mh := sha256.Sum256(raw)
	records, e := s.sqlRecords()
	if e != nil {
		return e
	}
	tx, e := s.sql.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if mh != s.sql.metadata {
		if _, e = tx.Exec(`INSERT INTO vetra_metadata(key,payload) VALUES('state',$1) ON CONFLICT(key) DO UPDATE SET payload=excluded.payload`, packed(raw)); e != nil {
			return e
		}
	}
	next := map[string][32]byte{}
	for _, r := range records {
		key := r.kind + ":" + r.id
		hash := sha256.Sum256(r.raw)
		next[key] = hash
		if old, ok := s.sql.fingerprints[key]; ok && old == hash {
			continue
		}
		if _, e = tx.Exec(`INSERT INTO vetra_records(kind,id,at,service,trace,payload) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(kind,id) DO UPDATE SET at=excluded.at,service=excluded.service,trace=excluded.trace,payload=excluded.payload`, r.kind, r.id, r.at, r.service, r.trace, packed(r.raw)); e != nil {
			return e
		}
	}
	for key := range s.sql.fingerprints {
		if _, ok := next[key]; !ok {
			kind, id := splitRecordKey(key)
			if _, e = tx.Exec(`DELETE FROM vetra_records WHERE kind=$1 AND id=$2`, kind, id); e != nil {
				return e
			}
		}
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	s.sql.metadata = mh
	s.sql.fingerprints = next
	return nil
}
func splitRecordKey(k string) (string, string) {
	for i, c := range k {
		if c == ':' {
			return k[:i], k[i+1:]
		}
	}
	return k, ""
}
func (s *Store) Close() error {
	if s.sql != nil {
		return s.sql.db.Close()
	}
	return nil
}
func (s *Store) StorageInfo() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	engine := "json"
	if s.sql != nil {
		engine = s.sql.engine
	}
	var disk int64
	if engine == "sqlite" {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			if info, e := os.Stat(s.sql.path + suffix); e == nil {
				disk += info.Size()
			}
		}
	}
	return map[string]any{"retention_hours": 24, "disk_bytes": disk, "span_limit": MaxStoredSpans, "log_limit": MaxLogs, "metric_limit": MaxMetricPoints, "rum_event_limit": rumMaxEvents, "replay_byte_limit": rumReplayBytes, "engine": engine, "incremental": s.sql != nil, "compressed_records": s.sql != nil, "spans": len(s.data.Spans), "logs": len(s.data.Logs), "metrics": len(s.data.MetricPoints), "rum_events": len(s.data.RUMEvents), "replay_chunks": len(s.data.RUMReplay), "query_mode": "bounded-hot-window", "distributed": false}
}

func (s *Store) ExportJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.data)
}
