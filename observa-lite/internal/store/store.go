package store

import (
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type persisted struct {
	InventorySettings  map[string]InventorySettings       `json:"inventory_settings,omitempty"`
	Integrations       map[string]model.Integration       `json:"integrations,omitempty"`
	IntegrationResults map[string]model.IntegrationResult `json:"integration_results,omitempty"`
	APMRules           map[string]model.APMRule           `json:"apm_rules,omitempty"`
	Problems           []model.RuleProblem                `json:"rule_problems,omitempty"`
	SLOs               map[string]model.SLOPolicy         `json:"slos,omitempty"`
	Logs               []model.LogRecord                  `json:"logs,omitempty"`
	MetricPoints       []model.MetricPoint                `json:"metric_points,omitempty"`
	Spans              []model.Span                       `json:"spans,omitempty"`
	Hosts              map[string]model.Host              `json:"hosts"`
	Gateways           map[string]model.GatewayEndpoint   `json:"gateways,omitempty"`
	Templates          map[string]model.Template          `json:"templates"`
	Sources            map[string]model.DataSource        `json:"sources"`
	Dashboards         map[string]model.Dashboard         `json:"dashboards"`
	Events             []model.Event                      `json:"events"`
}
type Store struct {
	mu   sync.RWMutex
	file string
	data persisted
}

func New(file string) (*Store, error) {
	s := &Store{file: file, data: persisted{APMRules: map[string]model.APMRule{}, Problems: []model.RuleProblem{}, SLOs: map[string]model.SLOPolicy{}, Hosts: map[string]model.Host{}, Gateways: map[string]model.GatewayEndpoint{}, Templates: map[string]model.Template{}, Sources: map[string]model.DataSource{}, Dashboards: map[string]model.Dashboard{}, Events: []model.Event{}}}
	if b, e := os.ReadFile(file); e == nil {
		if err := json.Unmarshal(b, &s.data); err != nil {
			return nil, fmt.Errorf("read database: %w", err)
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if s.data.Integrations == nil {
		s.data.Integrations = map[string]model.Integration{}
	}
	if s.data.IntegrationResults == nil {
		s.data.IntegrationResults = map[string]model.IntegrationResult{}
	}
	if s.data.APMRules == nil {
		s.data.APMRules = map[string]model.APMRule{}
	}
	if s.data.SLOs == nil {
		s.data.SLOs = map[string]model.SLOPolicy{}
	}
	if s.data.Gateways == nil {
		s.data.Gateways = map[string]model.GatewayEndpoint{}
	}
	if s.data.Hosts == nil {
		s.data.Hosts = map[string]model.Host{}
	}
	if s.data.Templates == nil {
		s.data.Templates = map[string]model.Template{}
	}
	if s.data.Sources == nil {
		s.data.Sources = map[string]model.DataSource{}
	}
	if s.data.Dashboards == nil {
		s.data.Dashboards = map[string]model.Dashboard{}
	}
	return s, nil
}
func (s *Store) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.file), 0700); err != nil {
		return err
	}
	b, e := json.MarshalIndent(s.data, "", "  ")
	if e != nil {
		return e
	}
	tmp := s.file + ".tmp"
	if e := os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, s.file)
}
func (s *Store) AddEventLocked(entity, severity, summary string, evidence ...string) {
	for i := range s.data.Events {
		ev := &s.data.Events[i]
		if ev.Entity == entity && ev.Summary == summary && ev.Status == "open" {
			ev.Evidence = evidence
			return
		}
	}
	s.data.Events = append(s.data.Events, model.Event{ID: fmt.Sprintf("evt-%d", time.Now().UnixNano()), At: time.Now().UTC(), Entity: entity, Severity: severity, Summary: summary, Evidence: evidence, Status: "open"})
	if len(s.data.Events) > 300 {
		s.data.Events = s.data.Events[len(s.data.Events)-300:]
	}
}
func (s *Store) ResolveLocked(entity, summary string) {
	for i := range s.data.Events {
		e := &s.data.Events[i]
		if e.Entity == entity && e.Summary == summary && e.Status == "open" {
			e.Status = "resolved"
		}
	}
}
func (s *Store) Ingest(d model.Discovery) error {
	if strings.TrimSpace(d.HostID) == "" || len(d.HostID) > 128 || len(d.Processes) > 100000 || len(d.Connections) > 200000 || len(d.HostGroup) > 100 || len(d.NetworkZone) > 256 || len(d.MonitoringMode) > 40 {
		return fmt.Errorf("invalid or oversized discovery")
	}
	// Keep the wire contract stable when a host has zero processes or zero sockets.
	if d.Processes == nil {
		d.Processes = []model.Process{}
	}
	if d.Connections == nil {
		d.Connections = []model.Connection{}
	}
	if d.Addresses == nil {
		d.Addresses = []string{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// PID disappearance cannot establish an application outage. Do not synthesize APM alerts from process churn.
	s.data.Hosts[d.HostID] = model.Host{ID: d.HostID, Name: d.Hostname, OS: d.OS, Addresses: d.Addresses, LastSeen: time.Now().UTC(), Processes: d.Processes, Connections: d.Connections, HostGroup: d.HostGroup, NetworkZone: d.NetworkZone, MonitoringMode: d.MonitoringMode}
	return s.persist()
}
func (s *Store) PutTemplate(v model.Template) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.ID == "" {
		v.ID = fmt.Sprintf("tpl-%d", time.Now().UnixNano())
	}
	if v.IntervalSeconds < 10 {
		v.IntervalSeconds = 30
	}
	if v.TimeoutSeconds < 1 || v.TimeoutSeconds > 30 {
		v.TimeoutSeconds = 5
	}
	if v.Status == "" {
		v.Status = "unknown"
	}
	s.data.Templates[v.ID] = v
	return s.persist()
}
func (s *Store) DeleteTemplate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Templates, id)
	return s.persist()
}
func (s *Store) Templates() []model.Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []model.Template{}
	for _, v := range s.data.Templates {
		result = append(result, v)
	}
	return result
}
func (s *Store) TemplateResult(id string, at time.Time, ok bool, value float64, msg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, exists := s.data.Templates[id]
	if !exists {
		return nil
	}
	prev := t.Status
	t.LastRun = at
	t.LastValue = value
	t.LastError = msg
	if ok {
		t.Status = "ok"
	} else {
		t.Status = "critical"
	}
	s.data.Templates[id] = t
	if !ok && prev != "critical" {
		s.AddEventLocked("tpl:"+id, "critical", "Verificação do template falhou", msg, "alvo: "+t.Target)
	}
	if ok {
		s.ResolveLocked("tpl:"+id, "Verificação do template falhou")
	}
	return s.persist()
}
func (s *Store) PutSource(v model.DataSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.ID == "" {
		v.ID = fmt.Sprintf("src-%d", time.Now().UnixNano())
	}
	s.data.Sources[v.ID] = v
	return s.persist()
}
func (s *Store) Source(id string) (model.DataSource, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Sources[id]
	return v, ok
}
func (s *Store) Dashboard(id string) (model.Dashboard, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Dashboards[id]
	return cloneDashboard(v), ok
}
func (s *Store) Snapshot() model.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := model.Snapshot{GeneratedAt: time.Now().UTC(), Hosts: []model.Host{}, Templates: []model.Template{}, DataSources: []model.DataSource{}, Dashboards: []model.Dashboard{}, Events: append([]model.Event{}, s.data.Events...)}
	for _, h := range s.data.Hosts {
		// Normalize legacy JSON state as well as newly ingested snapshots.
		if h.Processes == nil {
			h.Processes = []model.Process{}
		}
		if h.Connections == nil {
			h.Connections = []model.Connection{}
		}
		if h.Addresses == nil {
			h.Addresses = []string{}
		}
		h = s.projectInventory(h)
		v.Hosts = append(v.Hosts, h)
	}
	for _, t := range s.data.Templates {
		v.Templates = append(v.Templates, t)
	}
	for _, d := range s.data.Sources {
		d.SecretEnv = ""
		v.DataSources = append(v.DataSources, d)
	}
	for _, d := range s.data.Dashboards {
		v.Dashboards = append(v.Dashboards, cloneDashboard(d))
	}
	sort.Slice(v.Hosts, func(i, j int) bool { return v.Hosts[i].Name < v.Hosts[j].Name })
	sort.Slice(v.Templates, func(i, j int) bool { return v.Templates[i].Name < v.Templates[j].Name })
	v.ProcessMap, v.NetworkMap = BuildMaps(v.Hosts)
	for i := range v.ProcessMap.Nodes {
		if v.ProcessMap.Nodes[i].Kind == "service" {
			v.ProcessMap.Nodes[i].Kind = "process-group"
		}
	}
	// Passive OS observations never become confirmed APM service entities.
	v.ServiceMap = apmMap(s.data.Spans, time.Now().Add(-time.Hour), time.Now())
	return v
}

// UpsertGateway registers only administrator-approved relay destinations.
func (s *Store) UpsertGateway(g model.GatewayEndpoint) error {
	if len(g.ID) < 1 || len(g.ID) > 80 || len(g.NetworkZone) < 1 || len(g.NetworkZone) > 120 ||
		g.Priority < 0 || g.Priority > 1000 {
		return fmt.Errorf("invalid gateway id, zone or priority")
	}
	for _, v := range []string{g.ID, g.NetworkZone} {
		for _, c := range v {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-') {
				return fmt.Errorf("invalid gateway or zone identifier")
			}
		}
	}
	u, err := url.Parse(g.URL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || len(g.URL) > 512 {
		return fmt.Errorf("gateway URL must be an HTTPS origin without path, query or credentials")
	}
	g.URL = "https://" + u.Host
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Gateways[g.ID] = g
	return s.persist()
}
func (s *Store) DeleteGateway(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Gateways, id)
	return s.persist()
}
func (s *Store) Gateways() []model.GatewayEndpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]model.GatewayEndpoint, 0, len(s.data.Gateways))
	for _, v := range s.data.Gateways {
		res = append(res, v)
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].Priority == res[j].Priority {
			return res[i].ID < res[j].ID
		}
		return res[i].Priority < res[j].Priority
	})
	return res
}
func (s *Store) ConnectionInfo(zone string, allowFallback bool) model.ConnectionInfo {
	if zone == "" {
		zone = "default"
	}
	result := model.ConnectionInfo{NetworkZone: zone, Endpoints: []string{}, ValidForSeconds: 60}
	seen := map[string]bool{}
	for _, g := range s.Gateways() {
		if !g.Enabled || g.NetworkZone != zone || seen[g.URL] {
			continue
		}
		result.Endpoints = append(result.Endpoints, g.URL)
		seen[g.URL] = true
	}
	if zone != "default" && allowFallback {
		for _, g := range s.Gateways() {
			if !g.Enabled || g.NetworkZone != "default" || seen[g.URL] {
				continue
			}
			result.Endpoints = append(result.Endpoints, g.URL)
			seen[g.URL] = true
			result.DefaultZoneFallback = true
		}
	}
	return result
}
