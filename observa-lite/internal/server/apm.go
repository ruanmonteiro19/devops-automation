package server

import (
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func apmWindow(r *http.Request) (time.Time, time.Time, error) {
	to := time.Now().UTC()
	from := to.Add(-time.Hour)
	var err error
	if v := r.URL.Query().Get("to"); v != "" {
		to, err = time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return from, to, fmt.Errorf("invalid to timestamp")
		}
		from = to.Add(-time.Hour)
	}
	if v := r.URL.Query().Get("from"); v != "" {
		from, err = time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return from, to, fmt.Errorf("invalid from timestamp")
		}
	}
	if !from.Before(to) || to.Sub(from) > store.SpanRetention {
		return from, to, fmt.Errorf("window must be positive and at most 24 hours")
	}
	return from, to, nil
}
func (s *Server) apmServices(w http.ResponseWriter, r *http.Request) {
	from, to, err := apmWindow(r)
	if err != nil {
		fail(w, 400, err)
		return
	}
	write(w, 200, map[string]any{"services": s.Store.APMServices(from, to), "from": from, "to": to, "sampled_data": true, "retention_hours": 24, "max_spans": store.MaxStoredSpans})
}

func (s *Server) apmTimeseries(w http.ResponseWriter, r *http.Request) {
	from, to, err := apmWindow(r)
	if err != nil {
		fail(w, 400, err)
		return
	}
	series := s.Store.APMTimeseries(from, to)
	write(w, 200, map[string]any{"timeseries": series, "from": from, "to": to, "sampled_data": true, "series_limit": store.MaxTimeseriesServices, "truncated": len(s.Store.APMServices(from, to)) > len(series)})
}
func (s *Server) apmMap(w http.ResponseWriter, r *http.Request) {
	from, to, err := apmWindow(r)
	if err != nil {
		fail(w, 400, err)
		return
	}
	write(w, 200, s.Store.APMMap(from, to))
}
func (s *Server) apmTrace(w http.ResponseWriter, r *http.Request) {
	id := strings.ToLower(r.PathValue("id"))
	if t, ok := s.Store.APMTrace(id); ok {
		write(w, 200, t)
	} else {
		http.Error(w, "trace not found in retained spans", 404)
	}
}
func (s *Server) apmTraces(w http.ResponseWriter, r *http.Request) {
	from, to, err := apmWindow(r)
	if err != nil {
		fail(w, 400, err)
		return
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil || limit < 1 || limit > 200 {
			http.Error(w, "limit must be 1..200", 400)
			return
		}
	}
	failedOnly := r.URL.Query().Get("failed") == "true"
	service := r.URL.Query().Get("service")
	matches := []model.APMTrace{}
	for _, t := range s.Store.APMTraces(from, to) {
		serviceMatch := service == ""
		errorMatch := !failedOnly
		for _, sp := range t.Spans {
			// Apply predicates to the same span within the requested window.
			if sp.StartTime.Before(from) || sp.StartTime.After(to) {
				continue
			}
			if service != "" && service != sp.ServiceName && service != "svc:"+url.QueryEscape(sp.ServiceNamespace)+":"+url.QueryEscape(sp.ServiceName) {
				continue
			}
			serviceMatch = true
			if store.SpanFailed(sp) {
				errorMatch = true
			}
		}
		if serviceMatch && errorMatch {
			matches = append(matches, t)
		}
	}
	total := len(matches)
	if total > limit {
		matches = matches[:limit]
	}
	write(w, 200, map[string]any{"traces": matches, "total": total, "truncated": total > limit, "from": from, "to": to})
}
