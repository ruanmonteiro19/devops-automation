package store

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ruanmonteiro19/observa-lite/internal/model"
)

func TestAPMAnalysisSemanticConventionsAndEvidence(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	root := testSpan("1111111111111111", "", "orders", "server")
	root.ServiceNamespace = "prod"
	root.ServiceInstanceID = "replica-1"
	root.ResourceAttributes = map[string]any{"service.version": "v2"}
	root.Status = "error"
	root.Attributes = map[string]any{"http.route": "/orders/{id}", "http.request.method": "GET", "http.response.status_code": float64(503)}
	root.Events = []model.SpanEvent{{Name: "exception", Attributes: map[string]any{"exception.type": "Timeout", "exception.message": "upstream deadline"}}}
	modern := testSpan("2222222222222222", root.SpanID, "orders", "client")
	modern.ServiceNamespace = "prod"
	modern.Attributes = map[string]any{"db.system.name": "postgresql", "db.namespace": "inventory", "db.operation.name": "SELECT", "server.address": "database.internal", "server.port": float64(5432)}
	legacy := modern
	legacy.SpanID = "3333333333333333"
	legacy.Attributes = map[string]any{"db.system": "postgresql", "db.name": "inventory", "db.operation": "SELECT", "net.peer.name": "database.internal", "net.peer.port": "5432"}
	http := modern
	http.SpanID = "4444444444444444"
	http.Attributes = map[string]any{"http.url": "https://user:password@example.com/pay?token=secret", "http.method": "POST"}
	msg := modern
	msg.SpanID = "5555555555555555"
	msg.Kind = "producer"
	msg.Attributes = map[string]any{"messaging.system": "kafka", "messaging.destination.name": "orders", "messaging.operation.name": "send"}
	other := root
	other.ServiceNamespace = "test"
	other.SpanID = "6666666666666666"
	if e := s.IngestSpans([]model.Span{root, modern, legacy, http, msg, other}); e != nil {
		t.Fatal(e)
	}
	a := s.APMAnalysis(serviceID(root), time.Time{}, time.Time{})
	if a.ObservedSpans != 5 || len(a.Endpoints) != 1 || a.Endpoints[0].Count != 1 {
		t.Fatalf("request counting %+v", a)
	}
	if len(a.DatabaseCalls) != 1 || a.DatabaseCalls[0].Count != 2 || a.DatabaseCalls[0].MeanDurationMS != 100 {
		t.Fatalf("database groups %+v", a.DatabaseCalls)
	}
	if len(a.OutboundCalls) != 2 || len(a.Messaging) != 1 || len(a.Failures) != 1 || a.Failures[0].Errors != 1 || a.Failures[0].Attributes["exception.type"] != "Timeout" {
		t.Fatalf("analysis %+v", a)
	}
	if a.Instances[0].Name != "replica-1" || a.Versions[0].Name != "v2" {
		t.Fatal("missing resource evidence")
	}
	encoded, _ := json.Marshal(a)
	if strings.Contains(string(encoded), "password") || strings.Contains(string(encoded), "token=") || strings.Contains(string(encoded), "secret") {
		t.Fatal("raw sensitive outbound URL leaked")
	}
	if len(a.DatabaseCalls[0].ExampleTraceIDs) != 1 {
		t.Fatal("duplicate trace evidence")
	}
}
func TestAPMAnalysisNoInventedDatabaseAndExamplesBound(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	spans := []model.Span{}
	for i := 0; i < 7; i++ {
		v := testSpan("1111111111111111", "", "web", "client")
		v.TraceID = strings.Repeat(string(rune('1'+i)), 32)
		v.Attributes = map[string]any{"server.address": "mysql.internal", "server.port": 3306}
		spans = append(spans, v)
	}
	if e := s.IngestSpans(spans); e != nil {
		t.Fatal(e)
	}
	a := s.APMAnalysis(serviceID(spans[0]), time.Time{}, time.Time{})
	if len(a.DatabaseCalls) != 0 || len(a.OutboundCalls) != 1 || a.OutboundCalls[0].Count != 7 || len(a.OutboundCalls[0].ExampleTraceIDs) != 5 {
		t.Fatalf("invented identity or unbounded examples %+v", a)
	}
	if len(a.Endpoints) != 0 {
		t.Fatal("client calls counted as inbound endpoints")
	}
}
func TestAPMAnalysisEmptyAndTimeframe(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	v := testSpan("1111111111111111", "", "web", "server")
	if e := s.IngestSpans([]model.Span{v}); e != nil {
		t.Fatal(e)
	}
	a := s.APMAnalysis(serviceID(v), time.Now().Add(-time.Second), time.Now())
	if a.ObservedSpans != 0 || a.Failures == nil || a.DatabaseCalls == nil {
		t.Fatalf("empty timeframe %+v", a)
	}
}
