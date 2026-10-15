package store

import (
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testSpan(id, parent, service, kind string) model.Span {
	now := time.Now().UTC().Add(-time.Minute)
	return model.Span{TraceID: "11111111111111111111111111111111", SpanID: id, ParentSpanID: parent, ServiceName: service, Name: "GET /orders", Kind: kind, StartTime: now, EndTime: now.Add(100 * time.Millisecond), Status: "ok"}
}
func TestAPMRealLinksREDAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, e := New(path)
	if e != nil {
		t.Fatal(e)
	}
	root := testSpan("1111111111111111", "", "frontend", "server")
	client := testSpan("2222222222222222", root.SpanID, "frontend", "client")
	server := testSpan("3333333333333333", client.SpanID, "orders", "server")
	server.Status = "error"
	server.ResourceAttributes = map[string]any{"service.namespace": "prod"}
	if e = s.IngestSpans([]model.Span{root, client, server, server}); e != nil {
		t.Fatal(e)
	}
	services := s.APMServices(time.Time{}, time.Time{})
	if len(services) != 2 {
		t.Fatalf("services: %+v", services)
	}
	for _, v := range services {
		if v.Requests != 1 {
			t.Fatalf("double counted: %+v", v)
		}
		if v.Name == "orders" && (v.ErrorRate != 100 || v.Namespace != "prod") {
			t.Fatalf("metrics: %+v", v)
		}
	}
	m := s.APMMap(time.Time{}, time.Time{})
	if len(m.Edges) != 1 || m.Edges[0].Kind != "span-parent" {
		t.Fatalf("map: %+v", m)
	}
	reopened, e := New(path)
	if e != nil {
		t.Fatal(e)
	}
	trace, ok := reopened.APMTrace(root.TraceID)
	if !ok || len(trace.Spans) != 3 || trace.Incomplete {
		t.Fatalf("trace: %+v", trace)
	}
	// Returned maps must not expose internal persisted state.
	trace.Spans[2].ResourceAttributes = map[string]any{"changed": true}
	again, _ := reopened.APMTrace(root.TraceID)
	if len(again.Spans) != 3 {
		t.Fatal("mutated internal state")
	}
}
func TestAPMInvalidBatchAndFailedPersistAreAtomic(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	v := testSpan("1111111111111111", "", "web", "server")
	if e := s.IngestSpans([]model.Span{v}); e != nil {
		t.Fatal(e)
	}
	invalid := v
	invalid.SpanID = "bad"
	if e := s.IngestSpans([]model.Span{v, invalid}); e == nil {
		t.Fatal("accepted invalid span")
	}
	s.file = filepath.Join(t.TempDir(), "missing", "state") // A directory at the target prevents atomic rename.
	if e := os.MkdirAll(s.file, 0700); e != nil {
		t.Fatal(e)
	}
	v.SpanID = "2222222222222222"
	if e := s.IngestSpans([]model.Span{v}); e == nil {
		t.Fatal("expected persistence failure")
	}
	trace, _ := s.APMTrace(v.TraceID)
	if len(trace.Spans) != 1 {
		t.Fatal("failed write changed memory")
	}
}
func TestAPMMissingParentAndNamespaceIsolation(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	a := testSpan("1111111111111111", "2222222222222222", "web", "server")
	b := testSpan("3333333333333333", "", "web", "server")
	b.ServiceNamespace = "other"
	if e := s.IngestSpans([]model.Span{a, b}); e != nil {
		t.Fatal(e)
	}
	tr, _ := s.APMTrace(a.TraceID)
	if !tr.Incomplete || len(tr.MissingParentIDs) != 1 {
		t.Fatalf("completeness %+v", tr)
	}
	if len(s.APMServices(time.Time{}, time.Time{})) != 2 {
		t.Fatal("merged namespaces")
	}
	if len(s.APMMap(time.Time{}, time.Time{}).Edges) != 0 {
		t.Fatal("invented dependency")
	}
}

func TestAPMRetentionAndTimeFilter(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	fresh := testSpan("1111111111111111", "", "web", "server")
	expired := testSpan("2222222222222222", "", "web", "server")
	expired.StartTime = time.Now().Add(-25 * time.Hour)
	expired.EndTime = expired.StartTime.Add(time.Second)
	if e := s.IngestSpans([]model.Span{fresh, expired}); e != nil {
		t.Fatal(e)
	}
	trace, _ := s.APMTrace(fresh.TraceID)
	if len(trace.Spans) != 1 {
		t.Fatal("expired span retained")
	}
	if len(s.APMServices(time.Now().Add(-10*time.Second), time.Now())) != 0 {
		t.Fatal("time filter ignored")
	}
}

func TestAPMQueriesHideExpiredIdleState(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	v := testSpan("1111111111111111", "", "web", "server")
	v.StartTime = time.Now().Add(-25 * time.Hour)
	v.EndTime = v.StartTime.Add(time.Second)
	s.data.Spans = []model.Span{v}
	from := time.Now().Add(-48 * time.Hour)
	to := time.Now()
	if _, ok := s.APMTrace(v.TraceID); ok {
		t.Fatal("expired trace exposed")
	}
	if len(s.APMTraces(from, to)) != 0 || len(s.APMServices(from, to)) != 0 || len(s.APMMap(from, to).Nodes) != 0 {
		t.Fatal("expired idle data exposed")
	}
}

func TestAPMTimeseriesBucketsAndRequestKinds(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	from := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	to := from.Add(time.Hour)
	first := testSpan("1111111111111111", "", "web", "server")
	first.StartTime = from
	first.EndTime = from.Add(100 * time.Millisecond)
	boundary := testSpan("2222222222222222", "", "web", "server")
	boundary.StartTime = from.Add(time.Minute)
	boundary.EndTime = boundary.StartTime.Add(200 * time.Millisecond)
	boundary.Status = "error"
	last := testSpan("3333333333333333", "", "web", "consumer")
	last.StartTime = to
	last.EndTime = to.Add(time.Millisecond)
	client := testSpan("4444444444444444", "", "web", "client")
	client.StartTime = from
	client.EndTime = from.Add(time.Millisecond)
	if e := s.IngestSpans([]model.Span{first, boundary, last, client}); e != nil {
		t.Fatal(e)
	}
	series := s.APMTimeseries(from, to)
	if len(series) != 1 || len(series[0].Buckets) != 60 {
		t.Fatalf("series %+v", series)
	}
	b := series[0].Buckets
	total := 0
	for _, v := range b {
		total += v.Requests
	}
	if total != 3 || b[0].Requests != 1 || b[1].Requests != 1 || b[59].Requests != 1 {
		t.Fatalf("boundary/request count total=%d first=%+v second=%+v last=%+v", total, b[0], b[1], b[59])
	}
	if b[1].Errors != 1 || b[1].ErrorRate != 100 || b[1].P95MS != 200 || b[0].RequestsPerSecond != 1.0/60 {
		t.Fatalf("metrics %+v", b[1])
	}
	if b[2].Requests != 0 || b[2].P50MS != 0 {
		t.Fatal("nonempty zero bucket")
	}
	if len(s.APMTimeseries(to, from)) != 0 {
		t.Fatal("invalid timeframe accepted")
	}
}

func TestAPMMapKeepsParentContextAcrossWindow(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	parent := testSpan("1111111111111111", "", "frontend", "client")
	child := testSpan("2222222222222222", parent.SpanID, "backend", "server")
	child.StartTime = parent.StartTime.Add(time.Second)
	child.EndTime = child.StartTime.Add(time.Millisecond)
	if e := s.IngestSpans([]model.Span{parent, child}); e != nil {
		t.Fatal(e)
	}
	m := s.APMMap(child.StartTime, child.EndTime)
	if len(m.Edges) != 1 || len(m.Nodes) != 2 {
		t.Fatalf("lost parent context %+v", m)
	}
	for _, n := range m.Nodes {
		if n.Status != "unknown" {
			t.Fatal("inferred health")
		}
	}
}
func TestAPMIdentityAndCloning(t *testing.T) {
	a := testSpan("1111111111111111", "", "c", "server")
	a.ServiceNamespace = "a:b"
	b := a
	b.ServiceNamespace = "a"
	b.ServiceName = "b:c"
	if serviceID(a) == serviceID(b) {
		t.Fatal("identity collision")
	}
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	a.Attributes = map[string]any{"nested": map[string]any{"x": "original"}}
	if e := s.IngestSpans([]model.Span{a}); e != nil {
		t.Fatal(e)
	}
	a.Attributes["nested"].(map[string]any)["x"] = "caller mutation"
	tr, _ := s.APMTrace(a.TraceID)
	if tr.Spans[0].Attributes["nested"].(map[string]any)["x"] != "original" {
		t.Fatal("caller aliases store")
	}
	tr.Spans[0].Attributes["nested"].(map[string]any)["x"] = "reader mutation"
	tr, _ = s.APMTrace(a.TraceID)
	if tr.Spans[0].Attributes["nested"].(map[string]any)["x"] != "original" {
		t.Fatal("reader aliases store")
	}
}
func TestAPMTimeseriesCardinalityBound(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	spans := []model.Span{}
	for i := 0; i < MaxTimeseriesServices+1; i++ {
		v := testSpan(fmt.Sprintf("%016x", i+1), "", fmt.Sprintf("service-%03d", i), "server")
		spans = append(spans, v)
	}
	if e := s.IngestSpans(spans); e != nil {
		t.Fatal(e)
	}
	series := s.APMTimeseries(time.Time{}, time.Time{})
	if len(series) != MaxTimeseriesServices {
		t.Fatalf("unbounded cardinality %d", len(series))
	}
}

func TestAPMMapExplicitDependenciesAndPairedSuppression(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	db := testSpan("1111111111111111", "", "web", "client")
	db.ServiceNamespace = "prod"
	db.Attributes = map[string]any{"db.system.name": "postgresql", "db.namespace": "inventory", "server.address": "database.internal", "server.port": float64(5432)}
	legacy := db
	legacy.SpanID = "2222222222222222"
	legacy.ServiceName = "worker"
	legacy.Attributes = map[string]any{"db.system": "postgresql", "db.name": "inventory", "net.peer.name": "database.internal", "net.peer.port": "5432"}
	queue := db
	queue.SpanID = "3333333333333333"
	queue.Kind = "producer"
	queue.Attributes = map[string]any{"messaging.system": "kafka", "messaging.destination.name": "orders"}
	remote := db
	remote.SpanID = "4444444444444444"
	remote.Attributes = map[string]any{"http.url": "https://user:password@remote.example/path?token=secret"}
	paired := remote
	paired.SpanID = "5555555555555555"
	paired.Attributes = map[string]any{"server.address": "monitored.internal"}
	server := paired
	server.SpanID = "6666666666666666"
	server.ParentSpanID = paired.SpanID
	server.Kind = "server"
	server.ServiceName = "backend"
	portOnly := db
	portOnly.SpanID = "7777777777777777"
	portOnly.Attributes = map[string]any{"server.port": 3306}
	if e := s.IngestSpans([]model.Span{db, legacy, queue, remote, paired, server, portOnly}); e != nil {
		t.Fatal(e)
	}
	m := s.APMMap(time.Time{}, time.Time{})
	counts := map[string]int{}
	for _, n := range m.Nodes {
		counts[n.Kind]++
		if n.Status != "unknown" {
			t.Fatal("invented health")
		}
	}
	if counts["database"] != 1 || counts["messaging"] != 1 || counts["external"] != 1 {
		t.Fatalf("dependency nodes %+v", m.Nodes)
	}
	calls := 0
	for _, e := range m.Edges {
		if e.Kind == "observed-call" {
			calls += e.Count
			if !strings.Contains(e.Evidence, "trace_id=") {
				t.Fatal("missing trace evidence")
			}
		}
	}
	if calls != 4 {
		t.Fatalf("call count %d %+v", calls, m.Edges)
	}
	raw, _ := json.Marshal(m)
	for _, secret := range []string{"password", "secret", "token=", "/path", "monitored.internal"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("leaked/duplicated dependency %s", secret)
		}
	}
	other := db
	other.ServiceNamespace = "other"
	a, _, _ := dependencyNode(db)
	b, _, _ := dependencyNode(other)
	if a.ID == b.ID {
		t.Fatal("merged namespaces")
	}
}
func TestAPMMapPairedChildOutsideWindowSuppressesExternal(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	client := testSpan("1111111111111111", "", "web", "client")
	client.Attributes = map[string]any{"server.address": "backend.internal"}
	child := testSpan("2222222222222222", client.SpanID, "backend", "server")
	child.StartTime = client.StartTime.Add(time.Second)
	child.EndTime = child.StartTime.Add(time.Millisecond)
	if e := s.IngestSpans([]model.Span{client, child}); e != nil {
		t.Fatal(e)
	}
	m := s.APMMap(client.StartTime, client.EndTime)
	for _, n := range m.Nodes {
		if n.Kind == "external" {
			t.Fatal("monitored service duplicated as external")
		}
	}
}

func TestSpanContextLinksPersistWithoutParentEdges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	v := testSpan("1111111111111111", "", "consumer", "consumer")
	v.Flags, v.TraceState = 769, "vendor=value"
	v.Scope = model.InstrumentationScope{Name: "queue-sdk", Version: "2", Attributes: map[string]any{"library": "queue"}}
	v.Links = []model.SpanLink{{TraceID: strings.Repeat("AB", 16), SpanID: strings.Repeat("CD", 8), Attributes: map[string]any{"queue": "orders"}}}
	if err := s.IngestSpans([]model.Span{v}); err != nil {
		t.Fatal(err)
	}
	v.Links[0].Attributes["queue"] = "mutated"
	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	trace, ok := reopened.APMTrace(v.TraceID)
	if !ok || len(trace.Spans) != 1 {
		t.Fatal("trace missing")
	}
	got := trace.Spans[0]
	if got.Flags != 769 || got.TraceState != "vendor=value" || got.Scope.Name != "queue-sdk" || got.Links[0].TraceID != strings.Repeat("ab", 16) || got.Links[0].Attributes["queue"] != "orders" {
		t.Fatalf("context lost or mutated: %+v", got)
	}
	if len(reopened.APMMap(time.Time{}, time.Time{}).Edges) != 0 {
		t.Fatal("link fabricated parent edge")
	}
	v.Links[0].SpanID = strings.Repeat("0", 16)
	if err := reopened.IngestSpans([]model.Span{v}); err == nil {
		t.Fatal("invalid link accepted")
	}
}
