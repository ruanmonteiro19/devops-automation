package store

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/ruanmonteiro19/observa-lite/internal/model"
)

const MaxSignalBytes = 16 << 20
const SignalRetention = 24 * time.Hour
const MaxLogs = 10000
const MaxMetricPoints = 50000

type SignalValidationError struct{ Message string }

func (e *SignalValidationError) Error() string { return e.Message }
func signalInvalid(message string) error       { return &SignalValidationError{message} }
func validSignalTime(t time.Time) bool         { return !t.IsZero() && !t.After(time.Now().Add(5*time.Minute)) }
func cloneSignal[T any](v T) T {
	b, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(b, &out)
	return out
}
func retainSignals[T any](old, batch []T, limit int, at func(T) time.Time) []T {
	cutoff := time.Now().Add(-SignalRetention)
	index := map[[32]byte]T{}
	for _, list := range [][]T{old, batch} {
		for _, v := range list {
			if at(v).Before(cutoff) {
				continue
			}
			b, _ := json.Marshal(v)
			index[sha256.Sum256(b)] = v
		}
	}
	next := make([]T, 0, len(index))
	for _, v := range index {
		next = append(next, v)
	}
	sort.Slice(next, func(i, j int) bool {
		a, b := at(next[i]), at(next[j])
		if a.Equal(b) {
			x, _ := json.Marshal(next[i])
			y, _ := json.Marshal(next[j])
			return string(x) < string(y)
		}
		return a.Before(b)
	})
	if len(next) > limit {
		next = next[len(next)-limit:]
	}
	total := 2
	first := len(next)
	for i := len(next) - 1; i >= 0; i-- {
		b, _ := json.Marshal(next[i])
		if total+len(b)+1 > MaxSignalBytes {
			break
		}
		total += len(b) + 1
		first = i
	}
	return next[first:]
}
func validateSignalBatch[T any](batch []T, limit int) error {
	if len(batch) > limit {
		return signalInvalid("signal batch exceeds record limit")
	}
	b, err := json.Marshal(batch)
	if err != nil {
		return signalInvalid("invalid signal JSON")
	}
	if len(b) > MaxSignalBytes {
		return signalInvalid("signal batch exceeds 16 MiB")
	}
	for _, v := range batch {
		b, _ := json.Marshal(v)
		if len(b) > 256<<10 {
			return signalInvalid("signal record exceeds 256 KiB")
		}
	}
	return nil
}
func (s *Store) IngestLogs(batch []model.LogRecord) error {
	if err := validateSignalBatch(batch, MaxLogs); err != nil {
		return err
	}
	batch = cloneSignal(batch)
	for _, v := range batch {
		if !validSignalTime(v.Timestamp) || strings.TrimSpace(v.ServiceName) == "" || len(v.ServiceName) > 512 || v.SeverityNumber < 0 || v.SeverityNumber > 24 || (v.TraceID != "" && !validID(v.TraceID, 16)) || (v.SpanID != "" && !validID(v.SpanID, 8)) {
			return signalInvalid("invalid log identity, severity or timestamp")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.data.Logs
	s.data.Logs = retainSignals(old, batch, MaxLogs, func(v model.LogRecord) time.Time { return v.Timestamp })
	if err := s.persist(); err != nil {
		s.data.Logs = old
		return fmt.Errorf("persist logs: %w", err)
	}
	return nil
}
func (s *Store) IngestMetrics(batch []model.MetricPoint) error {
	if err := validateSignalBatch(batch, MaxMetricPoints); err != nil {
		return err
	}
	batch = cloneSignal(batch)
	for _, v := range batch {
		if !validSignalTime(v.Timestamp) || strings.TrimSpace(v.ServiceName) == "" || len(v.ServiceName) > 512 || strings.TrimSpace(v.Name) == "" || len(v.Name) > 1024 || (!v.StartTime.IsZero() && v.StartTime.After(v.Timestamp)) || v.Data == nil || (v.Kind != "gauge" && v.Kind != "sum" && v.Kind != "histogram") {
			return signalInvalid("invalid metric name, kind or timestamp")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.data.MetricPoints
	s.data.MetricPoints = retainSignals(old, batch, MaxMetricPoints, func(v model.MetricPoint) time.Time { return v.Timestamp })
	if err := s.persist(); err != nil {
		s.data.MetricPoints = old
		return fmt.Errorf("persist metrics: %w", err)
	}
	return nil
}
func signalWindow(t, from, to time.Time) bool {
	return !t.Before(time.Now().Add(-SignalRetention)) && !t.Before(from) && !t.After(to)
}
func (s *Store) SignalsLogs(from, to time.Time, traceID, service string, limit int) []model.LogRecord {
	from, to = normalizeWindow(from, to)
	if limit <= 0 || limit > MaxLogs {
		limit = 1000
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.LogRecord{}
	for i := len(s.data.Logs) - 1; i >= 0 && len(out) < limit; i-- {
		v := s.data.Logs[i]
		if signalWindow(v.Timestamp, from, to) && (traceID == "" || strings.EqualFold(traceID, v.TraceID)) && (signalServiceMatches(service, v.ServiceName, v.ResourceAttributes)) {
			out = append(out, v)
		}
	}
	return cloneSignal(out)
}
func (s *Store) SignalsMetrics(from, to time.Time, name, service string, limit int) []model.MetricPoint {
	from, to = normalizeWindow(from, to)
	if limit <= 0 || limit > MaxMetricPoints {
		limit = 1000
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.MetricPoint{}
	for i := len(s.data.MetricPoints) - 1; i >= 0 && len(out) < limit; i-- {
		v := s.data.MetricPoints[i]
		if signalWindow(v.Timestamp, from, to) && (name == "" || name == v.Name) && (signalServiceMatches(service, v.ServiceName, v.ResourceAttributes)) {
			out = append(out, v)
		}
	}
	return cloneSignal(out)
}

func signalServiceMatches(filter, name string, resource map[string]any) bool {
	if filter == "" || filter == name {
		return true
	}
	namespace, _ := resource["service.namespace"].(string)
	return filter == "svc:"+url.QueryEscape(namespace)+":"+url.QueryEscape(name)
}
