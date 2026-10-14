package store

import (
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type persisted struct {
	Hosts      map[string]model.Host       `json:"hosts"`
	Templates  map[string]model.Template   `json:"templates"`
	Sources    map[string]model.DataSource `json:"sources"`
	Dashboards map[string]model.Dashboard  `json:"dashboards"`
	Events     []model.Event               `json:"events"`
}
type Store struct {
	mu   sync.RWMutex
	file string
	data persisted
}

func New(file string) (*Store, error) {
	s := &Store{file: file, data: persisted{Hosts: map[string]model.Host{}, Templates: map[string]model.Template{}, Sources: map[string]model.DataSource{}, Dashboards: map[string]model.Dashboard{}, Events: []model.Event{}}}
	if b, e := os.ReadFile(file); e == nil {
		if err := json.Unmarshal(b, &s.data); err != nil {
			return nil, fmt.Errorf("read database: %w", err)
		}
	} else if !os.IsNotExist(e) {
		return nil, e
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
	if strings.TrimSpace(d.HostID) == "" || len(d.HostID) > 128 || len(d.Processes) > 100000 || len(d.Connections) > 200000 || len(d.HostGroup)>100 || len(d.NetworkZone)>256 || len(d.MonitoringMode)>40 {
		return fmt.Errorf("invalid or oversized discovery")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// PID disappearance cannot establish an application outage. Do not synthesize APM alerts from process churn.
	s.data.Hosts[d.HostID] = model.Host{ID: d.HostID, Name: d.Hostname, OS: d.OS, Addresses: d.Addresses, LastSeen: time.Now().UTC(), Processes: d.Processes, Connections: d.Connections,HostGroup:d.HostGroup,NetworkZone:d.NetworkZone,MonitoringMode:d.MonitoringMode}
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
func (s *Store) PutDashboard(v model.Dashboard) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.ID == "" {
		v.ID = fmt.Sprintf("dash-%d", time.Now().UnixNano())
	}
	s.data.Dashboards[v.ID] = v
	return s.persist()
}
func (s *Store) Dashboard(id string) (model.Dashboard, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Dashboards[id]
	return v, ok
}
func (s *Store) Snapshot() model.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := model.Snapshot{GeneratedAt: time.Now().UTC(), Hosts: []model.Host{}, Templates: []model.Template{}, DataSources: []model.DataSource{}, Dashboards: []model.Dashboard{}, Events: append([]model.Event{}, s.data.Events...)}
	for _, h := range s.data.Hosts {
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
		v.Dashboards = append(v.Dashboards, d)
	}
	sort.Slice(v.Hosts, func(i, j int) bool { return v.Hosts[i].Name < v.Hosts[j].Name })
	sort.Slice(v.Templates, func(i, j int) bool { return v.Templates[i].Name < v.Templates[j].Name })
	v.ProcessMap, v.NetworkMap = BuildMaps(v.Hosts)
 for i:=range v.ProcessMap.Nodes{
  if v.ProcessMap.Nodes[i].Kind=="service"{v.ProcessMap.Nodes[i].Kind="process-group"}
 }
 // Passive OS observations never become confirmed APM service entities.
 v.ServiceMap=model.Map{Nodes:[]model.Node{},Edges:[]model.Edge{}}
	return v
}
