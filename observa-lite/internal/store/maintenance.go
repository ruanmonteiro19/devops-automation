package store

import (
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"time"
)

func recent[T any](list []T, cutoff time.Time, at func(T) time.Time) []T {
	out := make([]T, 0, len(list))
	for _, v := range list {
		if !at(v).Before(cutoff) {
			out = append(out, v)
		}
	}
	return out
}

// PruneExpired applies time retention even when a source stops sending data.
func (s *Store) PruneExpired(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now.Sub(s.maintenanceAt) < time.Minute {
		return nil
	}
	old := s.data
	s.data.Spans = recent(old.Spans, now.Add(-SpanRetention), func(v model.Span) time.Time { return v.StartTime })
	s.data.Logs = recent(old.Logs, now.Add(-SignalRetention), func(v model.LogRecord) time.Time { return v.Timestamp })
	s.data.MetricPoints = recent(old.MetricPoints, now.Add(-SignalRetention), func(v model.MetricPoint) time.Time { return v.Timestamp })
	s.data.RUMEvents = recent(old.RUMEvents, now.Add(-24*time.Hour), func(v model.RUMEvent) time.Time { return v.At })
	s.data.RUMReplay = recent(old.RUMReplay, now.Add(-24*time.Hour), func(v model.RUMReplayChunk) time.Time { return v.At })
	sessions := map[string]LoginSession{}
	for k, v := range old.Sessions {
		if now.Before(v.Expires) {
			sessions[k] = v
		}
	}
	s.data.Sessions = sessions
	changed := len(old.Spans) != len(s.data.Spans) || len(old.Logs) != len(s.data.Logs) || len(old.MetricPoints) != len(s.data.MetricPoints) || len(old.RUMEvents) != len(s.data.RUMEvents) || len(old.RUMReplay) != len(s.data.RUMReplay) || len(old.Sessions) != len(sessions)
	if changed {
		if e := s.persist(); e != nil {
			s.data = old
			return e
		}
	}
	s.maintenanceAt = now
	return nil
}
