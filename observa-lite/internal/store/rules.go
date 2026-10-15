package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"math"
	"sort"
	"time"
)

var ErrRuleValidation = errors.New("invalid APM rule")
var ErrRuleLimit = errors.New("APM rule limit reached")
var ErrRuleNotFound = errors.New("APM rule not found")

const MaxAPMRules = 100
const MaxRuleProblems = 1000

func ValidateAPMRule(v model.APMRule) error {
	if err := ValidateSLOPolicy(model.SLOPolicy{ID: v.ID, Name: v.Name, ServiceID: v.ServiceID, Kind: "availability", TargetPercent: 99}); err != nil {
		return fmt.Errorf("%w: %v", ErrRuleValidation, err)
	}
	if (v.Metric != "error_rate" && v.Metric != "p95_ms") || math.IsNaN(v.Threshold) || math.IsInf(v.Threshold, 0) || v.Threshold < 0 || (v.Metric == "error_rate" && v.Threshold > 100) || v.MinRequests < 1 || v.MinRequests > 20000 || v.WindowMinutes < 1 || v.WindowMinutes > 60 || (v.Severity != "warning" && v.Severity != "critical") {
		return ErrRuleValidation
	}
	return nil
}
func cloneRuleProblems(v []model.RuleProblem) []model.RuleProblem {
	b, _ := json.Marshal(v)
	var out []model.RuleProblem
	json.Unmarshal(b, &out)
	if out == nil {
		out = []model.RuleProblem{}
	}
	return out
}
func (s *Store) APMRules() []model.APMRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.APMRule, 0, len(s.data.APMRules))
	for _, v := range s.data.APMRules {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func transition(p *model.RuleProblem, now time.Time, state, summary string) {
	p.Timeline = append(p.Timeline, model.RuleTransition{At: now, State: state, Value: p.Value, Summary: summary})
	if len(p.Timeline) > 100 {
		p.Timeline = append(p.Timeline[:1], p.Timeline[len(p.Timeline)-99:]...)
	}
}
func closeRuleProblems(problems []model.RuleProblem, id string, now time.Time, reason string) {
	for i := range problems {
		p := &problems[i]
		if p.RuleID == id && p.Status == "active" {
			p.Status = "resolved"
			closedAt := now
			if closedAt.Before(p.UpdatedAt) {
				closedAt = p.UpdatedAt
			}
			p.ResolvedAt = &closedAt
			p.UpdatedAt = closedAt
			p.Evidence = append(p.Evidence, reason)
			transition(p, closedAt, "resolved", reason)
		}
	}
}
func (s *Store) PutAPMRule(v model.APMRule) error {
	if err := ValidateAPMRule(v); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	oldRules, oldProblems := s.data.APMRules, s.data.Problems
	if _, ok := oldRules[v.ID]; !ok && len(oldRules) >= MaxAPMRules {
		return ErrRuleLimit
	}
	next := make(map[string]model.APMRule, len(oldRules)+1)
	for k, p := range oldRules {
		next[k] = p
	}
	next[v.ID] = v
	s.data.APMRules = next
	s.data.Problems = cloneRuleProblems(oldProblems)
	if previous, exists := oldRules[v.ID]; exists && v.Enabled && (previous.ServiceID != v.ServiceID || previous.Metric != v.Metric || previous.Threshold != v.Threshold || previous.MinRequests != v.MinRequests || previous.WindowMinutes != v.WindowMinutes) {
		closeRuleProblems(s.data.Problems, v.ID, time.Now().UTC(), "Rule conditions changed explicitly; previous incident closed")
	}
	if !v.Enabled {
		closeRuleProblems(s.data.Problems, v.ID, time.Now().UTC(), "Rule disabled explicitly")
	}
	if err := s.persist(); err != nil {
		s.data.APMRules = oldRules
		s.data.Problems = oldProblems
		return err
	}
	return nil
}
func (s *Store) DeleteAPMRule(id string) error {
	if !sloIDPattern.MatchString(id) {
		return ErrRuleValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	oldRules, oldProblems := s.data.APMRules, s.data.Problems
	if _, ok := oldRules[id]; !ok {
		return ErrRuleNotFound
	}
	next := make(map[string]model.APMRule, len(oldRules))
	for k, v := range oldRules {
		if k != id {
			next[k] = v
		}
	}
	s.data.APMRules = next
	s.data.Problems = cloneRuleProblems(oldProblems)
	closeRuleProblems(s.data.Problems, id, time.Now().UTC(), "Rule deleted explicitly")
	if err := s.persist(); err != nil {
		s.data.APMRules = oldRules
		s.data.Problems = oldProblems
		return err
	}
	return nil
}
func (s *Store) RuleProblems(status string) []model.RuleProblem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.RuleProblem{}
	for _, p := range s.data.Problems {
		if status == "" || p.Status == status {
			out = append(out, p)
		}
	}
	out = cloneRuleProblems(out)
	sort.Slice(out, func(i, j int) bool { return out[i].OpenedAt.After(out[j].OpenedAt) })
	return out
}

// EvaluateAPMRules uses observed retained request spans, not extrapolated traffic
// or inferred causal relationships. Low traffic never implies recovery.
func (s *Store) EvaluateAPMRules(now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.APMRules) == 0 {
		return nil
	}
	old := s.data.Problems
	next := cloneRuleProblems(old)
	ids := make([]string, 0, len(s.data.APMRules))
	for id := range s.data.APMRules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r := s.data.APMRules[id]
		if !r.Enabled {
			continue
		}
		from := now.Add(-time.Duration(r.WindowMinutes) * time.Minute)
		sample := []model.Span{}
		for _, span := range s.data.Spans {
			if isRequest(span) && serviceID(span) == r.ServiceID && inWindow(span, from, now) {
				sample = append(sample, span)
			}
		}
		active := -1
		stale := false
		for i := range next {
			if next[i].RuleID == id && now.Before(next[i].UpdatedAt) {
				stale = true
			}
			if next[i].RuleID == id && next[i].Status == "active" {
				active = i
			}
		}
		// A stale evaluation must not regress incident timestamps or close newer state.
		if stale {
			continue
		}
		if len(sample) < r.MinRequests {
			if active >= 0 {
				p := &next[active]
				p.UpdatedAt = now
				p.Evidence = []string{fmt.Sprintf("Unknown: insufficient observed requests (%d < %d); active incident preserved", len(sample), r.MinRequests), "Retained sampled spans; no uptime or causal inference"}
				if len(p.Timeline) == 0 || p.Timeline[len(p.Timeline)-1].State != "unknown" {
					transition(p, now, "unknown", "Insufficient request sample; no recovery asserted")
				}
			}
			continue
		}
		m := metrics(sample, float64(r.WindowMinutes*60))
		value := m.ErrorRate
		if r.Metric == "p95_ms" {
			value = m.P95MS
		}
		evidence := []string{fmt.Sprintf("%s = %.3f; threshold >= %.3f; requests %d in last %d minutes", r.Metric, value, r.Threshold, len(sample), r.WindowMinutes), "Retained server/consumer span sample; 24h / 20000 spans / 32 MiB limits; sampling may omit traffic; threshold detection only, no causal RCA"}
		traceIDs := []string{}
		seen := map[string]bool{}
		for _, span := range sample {
			if len(traceIDs) < 5 && !seen[span.TraceID] {
				seen[span.TraceID] = true
				traceIDs = append(traceIDs, span.TraceID)
			}
		}
		if value >= r.Threshold {
			if active < 0 {
				next = append(next, model.RuleProblem{ID: fmt.Sprintf("rule-%s-%d", id, now.UnixNano()), RuleID: id, ServiceID: r.ServiceID, Name: r.Name, Severity: r.Severity, Status: "active", OpenedAt: now})
				active = len(next) - 1
			}
			p := &next[active]
			p.Name = r.Name
			p.ServiceID = r.ServiceID
			p.Severity = r.Severity
			p.Value = value
			p.Threshold = r.Threshold
			p.Metric = r.Metric
			p.WindowMinutes = r.WindowMinutes
			p.UpdatedAt = now
			p.Evidence = evidence
			p.ExampleTraceIDs = traceIDs
			if len(p.Timeline) == 0 || p.Timeline[len(p.Timeline)-1].State != "active" {
				transition(p, now, "active", "Observed request threshold breached")
			}
		} else if active >= 0 {
			p := &next[active]
			p.Value = value
			p.Evidence = evidence
			p.ExampleTraceIDs = traceIDs
			closeRuleProblems(next, id, now, "Observed request metric recovered below threshold with sufficient sample")
		}
	}
	if len(next) > MaxRuleProblems {
		sort.SliceStable(next, func(i, j int) bool {
			if next[i].Status != next[j].Status {
				return next[i].Status == "active"
			}
			return next[i].OpenedAt.After(next[j].OpenedAt)
		})
		next = next[:MaxRuleProblems]
	}
	s.data.Problems = next
	if err := s.persist(); err != nil {
		s.data.Problems = old
		return err
	}
	return nil
}
