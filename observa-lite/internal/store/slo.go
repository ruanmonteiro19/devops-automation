package store

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ruanmonteiro19/observa-lite/internal/model"
)

const MaxSLOPolicies = 100

var ErrSLOValidation = errors.New("invalid SLO policy")
var ErrSLOLimit = errors.New("SLO policy limit reached")
var ErrSLONotFound = errors.New("SLO policy not found")

var sloIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func ValidateSLOPolicy(v model.SLOPolicy) error {
	if !sloIDPattern.MatchString(v.ID) || strings.TrimSpace(v.Name) == "" || len(v.Name) > 256 || !strings.HasPrefix(v.ServiceID, "svc:") || len(v.ServiceID) > 4096 {
		return fmt.Errorf("%w: invalid SLO identity or service", ErrSLOValidation)
	}
	parts := strings.Split(v.ServiceID, ":")
	if len(parts) != 3 {
		return fmt.Errorf("%w: invalid SLO service id", ErrSLOValidation)
	}
	namespace, namespaceErr := url.QueryUnescape(parts[1])
	name, nameErr := url.QueryUnescape(parts[2])
	if namespaceErr != nil || nameErr != nil || strings.TrimSpace(name) == "" || len(name) > 512 || len(namespace) > 512 {
		return fmt.Errorf("%w: invalid SLO service id", ErrSLOValidation)
	}
	if math.IsNaN(v.TargetPercent) || math.IsInf(v.TargetPercent, 0) || v.TargetPercent <= 0 || v.TargetPercent > 100 {
		return fmt.Errorf("%w: target_percent must be greater than zero and at most 100", ErrSLOValidation)
	}
	if v.Kind != "availability" && v.Kind != "latency" {
		return fmt.Errorf("%w: kind must be availability or latency", ErrSLOValidation)
	}
	if math.IsNaN(v.LatencyThresholdMS) || math.IsInf(v.LatencyThresholdMS, 0) || v.LatencyThresholdMS < 0 || (v.Kind == "latency" && v.LatencyThresholdMS <= 0) {
		return fmt.Errorf("%w: latency threshold must be positive for latency policies", ErrSLOValidation)
	}
	return nil
}

func (s *Store) SLOPolicies() []model.SLOPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.SLOPolicy, 0, len(s.data.SLOs))
	for _, v := range s.data.SLOs {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
func (s *Store) PutSLOPolicy(v model.SLOPolicy) error {
	if err := ValidateSLOPolicy(v); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.data.SLOs
	if _, exists := old[v.ID]; !exists && len(old) >= MaxSLOPolicies {
		return ErrSLOLimit
	}
	next := make(map[string]model.SLOPolicy, len(old)+1)
	for k, p := range old {
		next[k] = p
	}
	next[v.ID] = v
	s.data.SLOs = next
	if err := s.persist(); err != nil {
		s.data.SLOs = old
		return err
	}
	return nil
}
func (s *Store) DeleteSLOPolicy(id string) error {
	if !sloIDPattern.MatchString(id) {
		return fmt.Errorf("%w: invalid SLO id", ErrSLOValidation)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.data.SLOs
	if _, exists := old[id]; !exists {
		return ErrSLONotFound
	}
	next := make(map[string]model.SLOPolicy, len(old))
	for k, p := range old {
		if k != id {
			next[k] = p
		}
	}
	s.data.SLOs = next
	if err := s.persist(); err != nil {
		s.data.SLOs = old
		return err
	}
	return nil
}

// EvaluateSLOs counts only retained server/consumer spans whose start is in
// [from,to]. Sampling and retention can exclude traffic: these are observed
// request objectives, not a claim about full traffic or wall-clock uptime.
func (s *Store) EvaluateSLOs(from, to time.Time) []model.SLOEvaluation {
	from, to = normalizeWindow(from, to)
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.SLOEvaluation, 0, len(s.data.SLOs))
	for _, policy := range s.data.SLOs {
		v := model.SLOEvaluation{SLOPolicy: policy, From: from, To: to, Status: "unknown", Basis: "Retained server/consumer span sample; start time within inclusive query window; retention up to 24h, 20000 spans / 32 MiB. Sampling and retention may omit traffic. Not wall-clock uptime. Latency good requests include failures if their duration meets the threshold; availability uses span error or HTTP 5xx."}
		for _, span := range s.data.Spans {
			if !isRequest(span) || serviceID(span) != policy.ServiceID || !inWindow(span, from, to) {
				continue
			}
			v.Total++
			good := !SpanFailed(span)
			if policy.Kind == "latency" {
				good = float64(span.EndTime.Sub(span.StartTime))/float64(time.Millisecond) <= policy.LatencyThresholdMS
			}
			if good {
				v.Good++
			} else {
				v.Bad++
			}
		}
		if v.Total > 0 {
			success := float64(v.Good) / float64(v.Total) * 100
			remaining := success - policy.TargetPercent
			v.SuccessPercent = &success
			v.ErrorBudgetRemainingPercent = &remaining
			if policy.TargetPercent < 100 {
				burn := (100 - success) / (100 - policy.TargetPercent)
				v.BurnRate = &burn
			}
			v.Status = "met"
			if success < policy.TargetPercent {
				v.Status = "breached"
			}
		}
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
