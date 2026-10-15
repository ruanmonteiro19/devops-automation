package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/ruanmonteiro19/observa-lite/internal/model"
)

var ErrDashboardNotFound = errors.New("dashboard not found")
var ErrDashboardValidation = errors.New("invalid dashboard")
var ErrDashboardLimit = errors.New("dashboard limit reached")

const MaxDashboards = 100
const MaxDashboardBytes = 1 << 20

func cloneDashboard(v model.Dashboard) model.Dashboard {
	if v.Panels != nil {
		v.Panels = append([]model.Panel{}, v.Panels...)
	}
	return v
}
func dashboardInvalid(message string) error {
	return fmt.Errorf("%w: %s", ErrDashboardValidation, message)
}
func dashboardServiceID(id string) bool {
	parts := strings.Split(id, ":")
	if len(parts) != 3 || parts[0] != "svc" {
		return false
	}
	namespace, err := url.QueryUnescape(parts[1])
	if err != nil || url.QueryEscape(namespace) != parts[1] {
		return false
	}
	name, err := url.QueryUnescape(parts[2])
	return err == nil && strings.TrimSpace(name) != "" && len(name) <= 512 && url.QueryEscape(name) == parts[2]
}
func validateDashboard(v *model.Dashboard) error {
	if strings.TrimSpace(v.ID) == "" || len(v.ID) > 128 {
		return dashboardInvalid("id required, max 128 bytes")
	}
	if strings.TrimSpace(v.Title) == "" || len(v.Title) > 256 {
		return dashboardInvalid("title required, max 256 bytes")
	}
	if len(v.Panels) > 50 {
		return dashboardInvalid("max 50 panels")
	}
	ids := map[string]bool{}
	for i := range v.Panels {
		p := &v.Panels[i]
		if strings.TrimSpace(p.ID) == "" || len(p.ID) > 128 || ids[p.ID] {
			return dashboardInvalid("panel id required, unique, max 128 bytes")
		}
		ids[p.ID] = true
		if strings.TrimSpace(p.Title) == "" || len(p.Title) > 256 {
			return dashboardInvalid("panel title required, max 256 bytes")
		}
		if len(p.HTML)+len(p.CSS)+len(p.JS) > 100000 {
			return dashboardInvalid("panel HTML, CSS and JS exceed 100000 bytes")
		}
		p.Kind = strings.ToLower(p.Kind)
		switch p.Kind {
		case "html", "stat":
		case "apm-stat", "apm-series":
			if p.SourceID != "observa-apm" {
				return dashboardInvalid("APM panels require observa-apm source")
			}
			var config struct {
				ServiceID string `json:"service_id"`
				Metric    string `json:"metric"`
			}
			decoder := json.NewDecoder(strings.NewReader(p.Query))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&config); err != nil {
				return dashboardInvalid("invalid APM panel query")
			}
			var extra any
			if err := decoder.Decode(&extra); err != io.EOF {
				return dashboardInvalid("invalid APM panel query")
			}
			if !dashboardServiceID(config.ServiceID) {
				return dashboardInvalid("invalid APM service id")
			}
			switch config.Metric {
			case "requests", "error_rate", "p50_ms", "p95_ms", "p99_ms", "requests_per_second":
			default:
				return dashboardInvalid("unsupported APM metric")
			}
		default:
			return dashboardInvalid("unsupported panel kind")
		}
	}
	encoded, err := json.Marshal(v)
	if err != nil || len(encoded) > MaxDashboardBytes {
		return dashboardInvalid("dashboard exceeds 1 MiB")
	}
	return nil
}
func (s *Store) PutDashboard(v model.Dashboard) error {
	v = cloneDashboard(v)
	if v.ID == "" {
		v.ID = fmt.Sprintf("dash-%d", time.Now().UnixNano())
	}
	if err := validateDashboard(&v); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, exists := s.data.Dashboards[v.ID]
	if !exists && len(s.data.Dashboards) >= MaxDashboards {
		return ErrDashboardLimit
	}
	s.data.Dashboards[v.ID] = v
	if err := s.persist(); err != nil {
		if exists {
			s.data.Dashboards[v.ID] = old
		} else {
			delete(s.data.Dashboards, v.ID)
		}
		return err
	}
	return nil
}
func (s *Store) DeleteDashboard(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data.Dashboards[id]
	if !ok {
		return ErrDashboardNotFound
	}
	delete(s.data.Dashboards, id)
	if err := s.persist(); err != nil {
		s.data.Dashboards[id] = v
		return err
	}
	return nil
}
