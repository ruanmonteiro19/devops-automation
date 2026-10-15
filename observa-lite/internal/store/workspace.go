package store

import (
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/vql"
	"regexp"
	"sort"
	"strings"
	"time"
)

var dbEnv = regexp.MustCompile(`^OBSERVA_DB_[A-Z0-9_]+_DSN$`)

func ValidateWorkspaceObject(o model.WorkspaceObject) error {
	if !rumID.MatchString(o.ID) || strings.TrimSpace(o.Name) == "" || len(o.Name) > 128 {
		return fmt.Errorf("ID e nome válidos são obrigatórios")
	}
	switch o.Kind {
	case "notebook":
		if len(o.Cells) == 0 || len(o.Cells) > 20 {
			return fmt.Errorf("notebook deve ter 1..20 células")
		}
		seen := map[string]bool{}
		for _, c := range o.Cells {
			if !rumID.MatchString(c.ID) || seen[c.ID] || len(c.Content) > 8192 || (c.Kind != "query" && c.Kind != "markdown") {
				return fmt.Errorf("célula inválida")
			}
			seen[c.ID] = true
		}
	case "workflow":
		if len(o.Steps) < 1 || len(o.Steps) > 10 || o.IntervalSeconds < 0 || (o.IntervalSeconds > 0 && o.IntervalSeconds < 60) || o.IntervalSeconds > 86400 {
			return fmt.Errorf("workflow: 1..10 etapas, intervalo manual ou 60..86400 s")
		}
		seen := map[string]bool{}
		for _, step := range o.Steps {
			if !rumID.MatchString(step.ID) || seen[step.ID] || len(step.Name) > 100 {
				return fmt.Errorf("etapa inválida")
			}
			seen[step.ID] = true
			switch step.Kind {
			case "query":
				if _, e := vql.Execute(step.Query, nil); e != nil {
					return e
				}
			case "condition":
				if step.MinRows < 0 || step.MinRows > 15000 {
					return fmt.Errorf("condição inválida")
				}
			case "database_check", "vulnerability_scan":
				if !rumID.MatchString(step.TargetID) {
					return fmt.Errorf("selecione um perfil")
				}
			default:
				return fmt.Errorf("ação não suportada")
			}
		}
	case "database":
		if o.Engine != "postgresql" || !dbEnv.MatchString(o.CredentialEnv) {
			return fmt.Errorf("PostgreSQL requer variável OBSERVA_DB_NOME_DSN")
		}
	case "software":
		if len(o.Packages) < 1 || len(o.Packages) > 500 {
			return fmt.Errorf("inventário deve ter 1..500 pacotes")
		}
		for _, p := range o.Packages {
			if p.Name == "" || p.Version == "" || len(p.Name) > 256 || len(p.Version) > 128 {
				return fmt.Errorf("pacote inválido")
			}
			switch p.Ecosystem {
			case "npm", "PyPI", "Go", "Maven", "NuGet", "Packagist", "RubyGems", "crates.io":
			default:
				return fmt.Errorf("ecossistema não suportado: %s", p.Ecosystem)
			}
		}
	default:
		return fmt.Errorf("tipo de objeto inválido")
	}
	return nil
}
func cloneObject(o model.WorkspaceObject) model.WorkspaceObject {
	b, _ := json.Marshal(o)
	var out model.WorkspaceObject
	_ = json.Unmarshal(b, &out)
	return out
}
func (s *Store) WorkspaceObjects(kind string) []model.WorkspaceObject {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.WorkspaceObject{}
	for _, o := range s.data.Workspace {
		if kind == "" || o.Kind == kind {
			out = append(out, cloneObject(o))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func (s *Store) WorkspaceObject(id string) (model.WorkspaceObject, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.data.Workspace[id]
	return cloneObject(o), ok
}
func (s *Store) PutWorkspace(o model.WorkspaceObject) error {
	if e := ValidateWorkspaceObject(o); e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Workspace == nil {
		s.data.Workspace = map[string]model.WorkspaceObject{}
	}
	old, exists := s.data.Workspace[o.ID]
	if exists && old.Kind != o.Kind {
		return fmt.Errorf("ID já pertence a outro tipo de objeto")
	}
	if !exists && len(s.data.Workspace) >= 200 {
		return fmt.Errorf("limite de 200 objetos")
	}
	o.UpdatedAt = time.Now().UTC()
	s.data.Workspace[o.ID] = cloneObject(o)
	if e := s.persist(); e != nil {
		if exists {
			s.data.Workspace[o.ID] = old
		} else {
			delete(s.data.Workspace, o.ID)
		}
		return e
	}
	return nil
}
func (s *Store) DeleteWorkspace(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.data.Workspace[id]
	if !ok {
		return fmt.Errorf("objeto não encontrado")
	}
	for _, o := range s.data.Workspace {
		if o.Kind == "workflow" && o.ID != id {
			for _, step := range o.Steps {
				if step.TargetID == id {
					return fmt.Errorf("objeto é usado pelo workflow %s", o.Name)
				}
			}
		}
	}
	delete(s.data.Workspace, id)
	if e := s.persist(); e != nil {
		s.data.Workspace[id] = old
		return e
	}
	return nil
}
func (s *Store) SaveOperation(run model.OperationRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, e := json.Marshal(run)
	if e != nil || len(b) > 3<<20 {
		return fmt.Errorf("resultado excede limite")
	}
	var copy model.OperationRun
	_ = json.Unmarshal(b, &copy)
	old := s.data.OperationRuns
	next := []model.OperationRun{}
	for _, r := range old {
		if r.ID != run.ID {
			next = append(next, r)
		}
	}
	next = append(next, copy)
	if len(next) > 100 {
		next = next[len(next)-100:]
	}
	for len(next) > 1 {
		encoded, _ := json.Marshal(next)
		if len(encoded) <= 8<<20 {
			break
		}
		next = next[1:]
	}
	s.data.OperationRuns = next
	if err := s.persist(); err != nil {
		s.data.OperationRuns = old
		return err
	}
	return nil
}
func (s *Store) OperationRuns(id string) []model.OperationRun {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.OperationRun{}
	for _, r := range s.data.OperationRuns {
		if id == "" || r.ObjectID == id {
			b, _ := json.Marshal(r)
			var copy model.OperationRun
			_ = json.Unmarshal(b, &copy)
			out = append(out, copy)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out
}
func (s *Store) SpanRows(from, to time.Time) []vql.Row {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []vql.Row{}
	for _, span := range s.data.Spans {
		if span.StartTime.Before(from) || span.StartTime.After(to) {
			continue
		}
		b, _ := json.Marshal(span)
		r := vql.Row{}
		_ = json.Unmarshal(b, &r)
		r["duration_ms"] = float64(span.EndTime.Sub(span.StartTime)) / float64(time.Millisecond)
		out = append(out, r)
	}
	return out
}
