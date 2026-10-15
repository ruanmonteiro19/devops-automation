package store

import (
	"errors"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testRule() model.APMRule {
	return model.APMRule{ID: "errors", Name: "Errors", ServiceID: "svc::api", Metric: "error_rate", Threshold: 50, MinRequests: 1, WindowMinutes: 5, Severity: "critical", Enabled: true}
}
func TestAPMRuleLifecycleDedupRecoveryRestart(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	s, _ := New(file)
	r := testRule()
	if err := s.PutAPMRule(r); err != nil {
		t.Fatal(err)
	}
	v := testSpan("1111111111111111", "", "api", "server")
	v.Status = "error"
	s.IngestSpans([]model.Span{v})
	now := time.Now().UTC()
	s.EvaluateAPMRules(now)
	s.EvaluateAPMRules(now.Add(time.Second))
	p := s.RuleProblems("active")
	if len(p) != 1 || p[0].Value != 100 || len(p[0].Timeline) != 1 {
		t.Fatalf("dedup %+v", p)
	}
	reopened, err := New(file)
	if err != nil || len(reopened.RuleProblems("active")) != 1 {
		t.Fatal("restart", err)
	}
	s.data.Spans = nil
	s.EvaluateAPMRules(now.Add(time.Second))
	p = s.RuleProblems("active")
	if len(p) != 1 || p[0].Timeline[len(p[0].Timeline)-1].State != "unknown" {
		t.Fatal("no data resolved active")
	}
	good := v
	good.SpanID = "2222222222222222"
	good.Status = "ok"
	good.StartTime = now.Add(-time.Second)
	good.EndTime = good.StartTime.Add(time.Millisecond)
	s.data.Spans = []model.Span{good}
	if err := s.EvaluateAPMRules(now.Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(s.RuleProblems("active")) != 0 || len(s.RuleProblems("resolved")) != 1 {
		t.Fatal("recovery missing")
	}
	// A new breach opens a new incident after a completed recovery.
	s.data.Spans = []model.Span{v}
	s.EvaluateAPMRules(now.Add(3 * time.Second))
	if len(s.RuleProblems("")) != 2 {
		t.Fatal("reopen did not retain history")
	}
	r.Enabled = false
	s.PutAPMRule(r)
	if len(s.RuleProblems("active")) != 0 {
		t.Fatal("disable failed to resolve")
	}
	r.Enabled = true
	s.PutAPMRule(r)
	s.EvaluateAPMRules(now.Add(4 * time.Second))
	s.DeleteAPMRule(r.ID)
	if len(s.RuleProblems("active")) != 0 || len(s.APMRules()) != 0 {
		t.Fatal("delete failed")
	}
}
func TestAPMRuleValidationLimitsAndRollback(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	r := testRule()
	bad := r
	bad.MinRequests = 0
	if !errors.Is(s.PutAPMRule(bad), ErrRuleValidation) {
		t.Fatal("invalid rule accepted")
	}
	if !errors.Is(s.DeleteAPMRule("missing"), ErrRuleNotFound) {
		t.Fatal("missing rule")
	}
	for i := 0; i < MaxAPMRules; i++ {
		v := r
		v.ID = fmt.Sprintf("r%d", i)
		s.data.APMRules[v.ID] = v
	}
	if !errors.Is(s.PutAPMRule(r), ErrRuleLimit) {
		t.Fatal("limit ignored")
	}
	s.data.APMRules = map[string]model.APMRule{}
	s.PutAPMRule(r)
	v := testSpan("1111111111111111", "", "api", "consumer")
	v.Status = "error"
	s.IngestSpans([]model.Span{v})
	s.EvaluateAPMRules(time.Now())
	broken := filepath.Join(t.TempDir(), "dir")
	os.Mkdir(broken, 0700)
	s.file = broken
	r.Enabled = false
	if s.PutAPMRule(r) == nil || !s.APMRules()[0].Enabled || len(s.RuleProblems("active")) != 1 {
		t.Fatal("failed disable not atomic")
	}
	if s.DeleteAPMRule(r.ID) == nil || len(s.APMRules()) != 1 || len(s.RuleProblems("active")) != 1 {
		t.Fatal("failed delete not atomic")
	}
	before := s.RuleProblems("active")[0].UpdatedAt
	if s.EvaluateAPMRules(time.Now().Add(time.Second)) == nil || !s.RuleProblems("active")[0].UpdatedAt.Equal(before) {
		t.Fatal("failed eval not atomic")
	}
}
func TestAPMRuleThresholdBoundaryAndTimelineBound(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	r := testRule()
	r.Metric = "p95_ms"
	r.Threshold = 100
	s.PutAPMRule(r)
	v := testSpan("1111111111111111", "", "api", "server")
	s.IngestSpans([]model.Span{v})
	s.EvaluateAPMRules(time.Now())
	if len(s.RuleProblems("active")) != 1 {
		t.Fatal("exact threshold must breach")
	}
	p := s.RuleProblems("active")[0]
	opened := p.Timeline[0]
	for i := 0; i < 150; i++ {
		transition(&p, time.Now(), "unknown", "sample")
	}
	if len(p.Timeline) != 100 || p.Timeline[0] != opened {
		t.Fatal("timeline bound lost opening")
	}
}

func TestAPMRuleStaleEvaluationAndProblemBounds(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	r := testRule()
	s.PutAPMRule(r)
	v := testSpan("1111111111111111", "", "api", "server")
	v.Status = "error"
	s.IngestSpans([]model.Span{v})
	now := time.Now().UTC()
	s.EvaluateAPMRules(now)
	original := s.RuleProblems("active")[0]
	s.EvaluateAPMRules(now.Add(-time.Second))
	if !s.RuleProblems("active")[0].UpdatedAt.Equal(original.UpdatedAt) {
		t.Fatal("stale evaluation regressed state")
	}
	for i := 0; i < 1100; i++ {
		s.data.Problems = append(s.data.Problems, model.RuleProblem{ID: fmt.Sprint(i), RuleID: "old", Status: "resolved", OpenedAt: now.Add(time.Duration(i) * time.Second)})
	}
	s.EvaluateAPMRules(now.Add(time.Second))
	if len(s.RuleProblems("")) != MaxRuleProblems || len(s.RuleProblems("active")) != 1 {
		t.Fatal("history bound evicted active incident")
	}
}
