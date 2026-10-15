package store

import (
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSignalsPersistenceDedupFiltersAndOwnership(t *testing.T) {
	file := filepath.Join(t.TempDir(), "signals.json")
	st, err := New(file)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-time.Minute)
	log := model.LogRecord{Timestamp: now, ServiceName: "api", TraceID: "11111111111111111111111111111111", Body: map[string]any{"message": "failed"}, Attributes: map[string]any{"key": "original"}, ResourceAttributes: map[string]any{"service.namespace": "shop"}}
	point := model.MetricPoint{Name: "requests", Kind: "sum", Temporality: "cumulative", Monotonic: true, Timestamp: now, ServiceName: "api", Data: map[string]any{"asInt": "9007199254740993"}}
	for i := 0; i < 2; i++ {
		if err = st.IngestLogs([]model.LogRecord{log}); err != nil {
			t.Fatal(err)
		}
		if err = st.IngestMetrics([]model.MetricPoint{point}); err != nil {
			t.Fatal(err)
		}
	}
	log.Attributes["key"] = "changed"
	point.Data["asInt"] = "0"
	reloaded, err := New(file)
	if err != nil {
		t.Fatal(err)
	}
	logs := reloaded.SignalsLogs(now.Add(-time.Second), now.Add(time.Second), log.TraceID, "api", 10)
	metrics := reloaded.SignalsMetrics(now.Add(-time.Second), now.Add(time.Second), "requests", "api", 10)
	if len(logs) != 1 || logs[0].Attributes["key"] != "original" || len(metrics) != 1 || metrics[0].Data["asInt"] != "9007199254740993" {
		t.Fatalf("persistence/dedup/ownership failed: %+v %+v", logs, metrics)
	}
	logs[0].Attributes["key"] = "changed again"
	metrics[0].Data["asInt"] = "2"
	if reloaded.SignalsLogs(time.Time{}, time.Time{}, "", "", 10)[0].Attributes["key"] != "original" || reloaded.SignalsMetrics(time.Time{}, time.Time{}, "", "", 10)[0].Data["asInt"] != "9007199254740993" {
		t.Fatal("query leaked mutable store data")
	}
	if len(reloaded.SignalsLogs(time.Time{}, time.Time{}, "not-found", "", 10)) != 0 || len(reloaded.SignalsMetrics(time.Time{}, time.Time{}, "requests", "other", 10)) != 0 {
		t.Fatal("signal filters crossed service/trace")
	}
}
func TestSignalsValidationAndPersistenceFailureAtomic(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "signals.json")
	st, err := New(file)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Second)
	valid := model.LogRecord{Timestamp: now, ServiceName: "api", Body: "first"}
	if err = st.IngestLogs([]model.LogRecord{valid}); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.SeverityNumber = 100
	if err = st.IngestLogs([]model.LogRecord{valid, invalid}); err == nil {
		t.Fatal("invalid batch accepted")
	}
	if len(st.SignalsLogs(time.Time{}, time.Time{}, "", "", 10)) != 1 {
		t.Fatal("invalid batch mutated store")
	}
	// A directory at the temporary-file path forces a real persistence failure.
	if err = os.Mkdir(file+".tmp", 0700); err != nil {
		t.Fatal(err)
	}
	valid.Body = "second"
	if err = st.IngestLogs([]model.LogRecord{valid}); err == nil {
		t.Fatal("disk failure ignored")
	}
	point := model.MetricPoint{Name: "gauge", Kind: "gauge", Timestamp: now, ServiceName: "api", Data: map[string]any{"asDouble": 1.0}}
	if err = st.IngestMetrics([]model.MetricPoint{point}); err == nil {
		t.Fatal("metric disk failure ignored")
	}
	if len(st.SignalsLogs(time.Time{}, time.Time{}, "", "", 10)) != 1 || len(st.SignalsMetrics(time.Time{}, time.Time{}, "", "", 10)) != 0 {
		t.Fatal("persistence failure mutated in-memory data")
	}
	if err = st.IngestLogs(make([]model.LogRecord, MaxLogs+1)); err == nil {
		t.Fatal("batch limit ignored")
	}
}
func TestSignalRetentionAndNewestCardinality(t *testing.T) {
	now := time.Now()
	old := model.LogRecord{Timestamp: now.Add(-25 * time.Hour), ServiceName: "api", Body: "expired"}
	batch := []model.LogRecord{{Timestamp: now.Add(-3 * time.Minute), Body: "first"}, {Timestamp: now.Add(-2 * time.Minute), Body: "second"}, {Timestamp: now.Add(-time.Minute), Body: "third"}}
	got := retainSignals([]model.LogRecord{old}, batch, 2, func(v model.LogRecord) time.Time { return v.Timestamp })
	if len(got) != 2 || got[0].Body != "second" || got[1].Body != "third" {
		t.Fatalf("retention/cardinality failed: %+v", got)
	}
}

func TestSignalLogicalServiceFilter(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "signals.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Second)
	for _, namespace := range []string{"shop", "admin"} {
		if err = st.IngestLogs([]model.LogRecord{{Timestamp: now, ServiceName: "api", ResourceAttributes: map[string]any{"service.namespace": namespace}, Body: "log"}}); err != nil {
			t.Fatal(err)
		}
		if err = st.IngestMetrics([]model.MetricPoint{{Timestamp: now, Name: "requests", ServiceName: "api", Kind: "gauge", Data: map[string]any{"asInt": "1"}, ResourceAttributes: map[string]any{"service.namespace": namespace}}}); err != nil {
			t.Fatal(err)
		}
	}
	if len(st.SignalsLogs(time.Time{}, time.Time{}, "", "svc:shop:api", 10)) != 1 || len(st.SignalsMetrics(time.Time{}, time.Time{}, "", "svc:admin:api", 10)) != 1 {
		t.Fatal("logical service filter crossed namespaces")
	}
}
