package server

import (
	"context"
	"errors"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"log"
	"net/http"
	"time"
)

func (s *Server) apmRules(w http.ResponseWriter, r *http.Request) {
	write(w, 200, map[string]any{"rules": s.Store.APMRules()})
}
func (s *Server) saveAPMRule(w http.ResponseWriter, r *http.Request) {
	var v model.APMRule
	if !decode(w, r, &v) {
		return
	}
	if err := s.Store.PutAPMRule(v); err != nil {
		ruleError(w, err)
		return
	}
	write(w, 200, map[string]bool{"saved": true})
}
func (s *Server) deleteAPMRule(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteAPMRule(r.PathValue("id")); err != nil {
		ruleError(w, err)
		return
	}
	write(w, 200, map[string]bool{"deleted": true})
}
func (s *Server) evaluateAPMRules(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.EvaluateAPMRules(time.Now().UTC()); err != nil {
		ruleError(w, err)
		return
	}
	write(w, 200, map[string]bool{"evaluated": true})
}
func (s *Server) apmProblems(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != "active" && status != "resolved" {
		http.Error(w, "status must be active or resolved", 400)
		return
	}
	write(w, 200, map[string]any{"problems": s.Store.RuleProblems(status), "detection": "configured_threshold_rules", "causal_rca": false})
}
func ruleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrRuleValidation):
		fail(w, 400, err)
	case errors.Is(err, store.ErrRuleLimit):
		fail(w, 429, err)
	case errors.Is(err, store.ErrRuleNotFound):
		fail(w, 404, err)
	default:
		http.Error(w, "rule storage failed", 500)
	}
}
func (s *Server) PollAPMRules(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		if len(s.Store.APMRules()) > 0 {
			if err := s.Store.EvaluateAPMRules(time.Now().UTC()); err != nil {
				log.Printf("APM rules persistence: %v", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) deleteDashboard(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteDashboard(r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrDashboardNotFound) {
			http.Error(w, "dashboard not found", 404)
		} else {
			http.Error(w, "dashboard storage failed", 500)
		}
		return
	}
	write(w, 200, map[string]bool{"deleted": true})
}
