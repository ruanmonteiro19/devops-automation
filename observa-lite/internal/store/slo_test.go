package store

import (
	"errors"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSLOEvaluationBoundariesAndIsolation(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	a := testSpan("1111111111111111", "", "api", "server")
	a.EndTime = a.StartTime.Add(100 * time.Millisecond)
	b := a
	b.SpanID = "2222222222222222"
	b.Status = "error"
	other := a
	other.SpanID = "3333333333333333"
	other.ServiceName = "other"
	other.Status = "error"
	client := a
	client.SpanID = "4444444444444444"
	client.Kind = "client"
	client.Status = "error"
	if err := s.IngestSpans([]model.Span{a, b, other, client}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []model.SLOPolicy{{ID: "availability", Name: "Available", ServiceID: serviceID(a), Kind: "availability", TargetPercent: 99}, {ID: "latency", Name: "Fast", ServiceID: serviceID(a), Kind: "latency", TargetPercent: 100, LatencyThresholdMS: 100}, {ID: "empty", Name: "Missing", ServiceID: "svc::missing", Kind: "availability", TargetPercent: 99}} {
		if err := s.PutSLOPolicy(p); err != nil {
			t.Fatal(err)
		}
	}
	results := s.EvaluateSLOs(a.StartTime, a.StartTime)
	if results[0].Total != 2 || results[0].Good != 1 || *results[0].SuccessPercent != 50 || *results[0].ErrorBudgetRemainingPercent != -49 || *results[0].BurnRate != 50 || results[0].Status != "breached" {
		t.Fatalf("availability: %+v", results[0])
	}
	if results[1].Status != "unknown" || results[1].SuccessPercent != nil || results[1].BurnRate != nil || results[1].ErrorBudgetRemainingPercent != nil {
		t.Fatalf("empty: %+v", results[1])
	}
	if results[2].Good != 2 || results[2].Status != "met" || results[2].BurnRate != nil || *results[2].ErrorBudgetRemainingPercent != 0 {
		t.Fatalf("latency: %+v", results[2])
	}
	p := results[0].SLOPolicy
	p.TargetPercent = 100
	s.PutSLOPolicy(p)
	r := s.EvaluateSLOs(a.StartTime, a.StartTime)[0]
	if r.BurnRate != nil || *r.ErrorBudgetRemainingPercent != -50 {
		t.Fatalf("zero budget: %+v", r)
	}
	reloaded, err := New(s.file)
	if err != nil || len(reloaded.SLOPolicies()) != 3 {
		t.Fatalf("persist: %v", err)
	}
}
func TestSLOValidationAndRollback(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	p := model.SLOPolicy{ID: "test", Name: "Test", ServiceID: "svc::api", Kind: "availability", TargetPercent: 99}
	if err := s.PutSLOPolicy(p); err != nil {
		t.Fatal(err)
	}
	for _, target := range []float64{0, -1, 101, math.NaN(), math.Inf(1)} {
		v := p
		v.TargetPercent = target
		if s.PutSLOPolicy(v) == nil {
			t.Fatal("accepted target", target)
		}
	}
	v := p
	v.Kind = "latency"
	if s.PutSLOPolicy(v) == nil {
		t.Fatal("accepted zero threshold")
	}
	v = p
	v.ID = "../bad"
	if s.PutSLOPolicy(v) == nil {
		t.Fatal("accepted id")
	}
	broken := filepath.Join(t.TempDir(), "directory")
	os.Mkdir(broken, 0700)
	s.file = broken
	v = p
	v.Name = "Changed"
	if s.PutSLOPolicy(v) == nil {
		t.Fatal("expected write failure")
	}
	if s.SLOPolicies()[0].Name != "Test" {
		t.Fatal("failed put mutated state")
	}
	if s.DeleteSLOPolicy(p.ID) == nil {
		t.Fatal("expected delete failure")
	}
	if len(s.SLOPolicies()) != 1 {
		t.Fatal("failed delete mutated state")
	}
}

func TestSLOLimitsAndRetention(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	// Populate directly to avoid 100 redundant persistence operations.
	s.data.SLOs = map[string]model.SLOPolicy{}
	for i := 0; i < MaxSLOPolicies; i++ {
		id := fmt.Sprintf("slo-%d", i)
		s.data.SLOs[id] = model.SLOPolicy{ID: id, Name: id, ServiceID: "svc::api", Kind: "availability", TargetPercent: 99}
	}
	p := model.SLOPolicy{ID: "extra", Name: "Extra", ServiceID: "svc::api", Kind: "availability", TargetPercent: 99}
	if s.PutSLOPolicy(p) == nil {
		t.Fatal("accepted policy beyond limit")
	}
	p.ID = "slo-0"
	if err := s.PutSLOPolicy(p); err != nil {
		t.Fatal("update at limit", err)
	}
	stale := testSpan("1111111111111111", "", "api", "server")
	stale.StartTime = time.Now().Add(-25 * time.Hour)
	stale.EndTime = stale.StartTime.Add(time.Second)
	s.data.Spans = []model.Span{stale}
	for _, result := range s.EvaluateSLOs(time.Now().Add(-48*time.Hour), time.Now()) {
		if result.Total != 0 || result.Status != "unknown" {
			t.Fatal("counted expired sample")
		}
	}
}

func TestSLOErrorClassification(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	if err := s.PutSLOPolicy(model.SLOPolicy{}); !errors.Is(err, ErrSLOValidation) {
		t.Fatalf("validation not classified: %v", err)
	}
	if err := s.DeleteSLOPolicy("missing"); !errors.Is(err, ErrSLONotFound) {
		t.Fatalf("not found not classified: %v", err)
	}
	if err := s.DeleteSLOPolicy("bad/id"); !errors.Is(err, ErrSLOValidation) {
		t.Fatalf("invalid id not classified: %v", err)
	}
	for i := 0; i < MaxSLOPolicies; i++ {
		id := fmt.Sprintf("slo-%d", i)
		s.data.SLOs[id] = model.SLOPolicy{ID: id}
	}
	if err := s.PutSLOPolicy(model.SLOPolicy{ID: "extra", Name: "Extra", ServiceID: "svc::api", Kind: "availability", TargetPercent: 99}); !errors.Is(err, ErrSLOLimit) {
		t.Fatalf("limit not classified: %v", err)
	}
}
