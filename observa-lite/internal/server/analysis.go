package server

import (
	"errors"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) apmAnalysis(w http.ResponseWriter, r *http.Request) {
	from, to, err := apmWindow(r)
	if err != nil {
		fail(w, 400, err)
		return
	}
	service := r.URL.Query().Get("service")
	if !strings.HasPrefix(service, "svc:") || len(service) > 4096 {
		http.Error(w, "logical service id required", 400)
		return
	}
	write(w, 200, s.Store.APMAnalysis(service, from, to))
}
func (s *Server) sloEvaluations(w http.ResponseWriter, r *http.Request) {
	from, to, err := apmWindow(r)
	if err != nil {
		fail(w, 400, err)
		return
	}
	write(w, 200, map[string]any{"slos": s.Store.EvaluateSLOs(from, to), "from": from, "to": to})
}
func (s *Server) saveSLO(w http.ResponseWriter, r *http.Request) {
	var v model.SLOPolicy
	if !decode(w, r, &v) {
		return
	}
	if err := s.Store.PutSLOPolicy(v); err != nil {
		sloError(w, err)
		return
	}
	write(w, 200, map[string]bool{"saved": true})
}
func (s *Server) deleteSLO(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteSLOPolicy(r.PathValue("id")); err != nil {
		sloError(w, err)
		return
	}
	write(w, 200, map[string]bool{"deleted": true})
}
func signalLimit(r *http.Request) (int, error) {
	v := r.URL.Query().Get("limit")
	if v == "" {
		return 100, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 200 {
		return 0, fmt.Errorf("limit must be 1..200")
	}
	return n, nil
}
func (s *Server) telemetryLogs(w http.ResponseWriter, r *http.Request) {
	from, to, err := apmWindow(r)
	if err != nil {
		fail(w, 400, err)
		return
	}
	limit, err := signalLimit(r)
	if err != nil {
		fail(w, 400, err)
		return
	}
	q := r.URL.Query()
	records := s.Store.SignalsLogs(from, to, q.Get("trace_id"), q.Get("service"), limit+1)
	truncated := len(records) > limit
	if truncated {
		records = records[:limit]
	}
	write(w, 200, map[string]any{"logs": records, "truncated": truncated, "from": from, "to": to, "storage": "local_bounded", "retention_hours": 24})
}
func (s *Server) telemetryMetrics(w http.ResponseWriter, r *http.Request) {
	from, to, err := apmWindow(r)
	if err != nil {
		fail(w, 400, err)
		return
	}
	limit, err := signalLimit(r)
	if err != nil {
		fail(w, 400, err)
		return
	}
	q := r.URL.Query()
	records := s.Store.SignalsMetrics(from, to, q.Get("name"), q.Get("service"), limit+1)
	truncated := len(records) > limit
	if truncated {
		records = records[:limit]
	}
	write(w, 200, map[string]any{"metrics": records, "truncated": truncated, "from": from, "to": to, "storage": "local_bounded", "retention_hours": 24})
}

func sloError(w http.ResponseWriter, err error) {
	code := 500
	switch {
	case errors.Is(err, store.ErrSLOValidation):
		code = 400
	case errors.Is(err, store.ErrSLOLimit):
		code = 429
	case errors.Is(err, store.ErrSLONotFound):
		code = 404
	}
	if code == 500 {
		http.Error(w, "policy storage failed", 500)
		return
	}
	fail(w, code, err)
}
