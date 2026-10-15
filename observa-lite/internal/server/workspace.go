package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"github.com/ruanmonteiro19/observa-lite/internal/vql"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func operationID() string { b := make([]byte, 12); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func rowsOf(v any) []vql.Row {
	b, _ := json.Marshal(v)
	out := []vql.Row{}
	_ = json.Unmarshal(b, &out)
	return out
}
func (s *Server) frontendCandidates() []vql.Row {
	out := []vql.Row{}
	for _, h := range s.Store.Snapshot().Hosts {
		for _, f := range h.Frontends {
			r := rowsOf([]model.FrontendCandidate{f})[0]
			r["host_name"] = h.Name
			r["fresh"] = time.Since(h.LastSeen) < 2*time.Minute
			r["addresses"] = h.Addresses
			r["rum"] = s.Store.RUMActivity(f.ID)
			out = append(out, r)
		}
	}
	return out
}
func (s *Server) databaseInventory() []vql.Row {
	out := []vql.Row{}
	seen := map[string]bool{}
	for _, h := range s.Store.Snapshot().Hosts {
		for _, p := range h.Processes {
			name := strings.ToLower(p.Name)
			engine := ""
			switch {
			case strings.Contains(name, "postgres") || name == "postmaster":
				engine = "postgresql"
			case strings.Contains(name, "mysqld") || strings.Contains(name, "mariadbd"):
				engine = "mysql"
			case strings.Contains(name, "sqlservr"):
				engine = "mssql"
			case strings.Contains(name, "mongod"):
				engine = "mongodb"
			case strings.Contains(name, "redis-server"):
				engine = "redis"
			case strings.HasPrefix(name, "ora_") || name == "oracle":
				engine = "oracle"
			}
			if engine == "" {
				continue
			}
			key := h.ID + ":" + engine
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, vql.Row{"id": key, "engine": engine, "host": h.Name, "service": p.Service, "source": "agent-process", "evidence": "processo observado; não comprova disponibilidade SQL", "last_seen": h.LastSeen})
		}
	}
	for _, r := range s.Store.SpanRows(time.Now().Add(-24*time.Hour), time.Now()) {
		attrs, _ := r["attributes"].(map[string]any)
		engine := attrs["db.system.name"]
		if engine == nil {
			engine = attrs["db.system"]
		}
		if engine == nil {
			continue
		}
		name := attrs["db.namespace"]
		if name == nil {
			name = attrs["db.name"]
		}
		host := attrs["server.address"]
		if host == nil {
			host = attrs["net.peer.name"]
		}
		key := fmt.Sprint(engine, ":", name, ":", host)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, vql.Row{"id": key, "engine": engine, "database": name, "host": host, "service": r["service_name"], "source": "otlp-db-attributes", "evidence": "atributos de span; sem inferência de conexão SQL saudável"})
	}
	return out
}
func (s *Server) workspaceRows(source string, minutes int) []vql.Row {
	from, to := time.Now().Add(-time.Duration(minutes)*time.Minute), time.Now()
	snap := s.Store.Snapshot()
	switch source {
	case "hosts":
		hosts := rowsOf(snap.Hosts)
		for _, h := range hosts {
			delete(h, "processes")
			delete(h, "connections")
			delete(h, "packages")
		}
		return hosts
	case "processes":
		rows := []vql.Row{}
		for _, h := range snap.Hosts {
			for _, p := range h.Processes {
				if model.IsSystemProcess(p) {
					continue
				}
				r := rowsOf([]model.Process{p})[0]
				r["host_id"] = h.ID
				r["host"] = h.Name
				rows = append(rows, r)
			}
		}
		return rows
	case "services":
		return rowsOf(s.Store.APMServices(from, to))
	case "spans":
		return s.Store.SpanRows(from, to)
	case "logs":
		return rowsOf(s.Store.SignalsLogs(from, to, "", "", 5000))
	case "metrics":
		return rowsOf(s.Store.SignalsMetrics(from, to, "", "", 5000))
	case "rum":
		return rowsOf(s.Store.RUMEvents("", from))
	case "events":
		return rowsOf(snap.Events)
	case "frontends":
		return s.frontendCandidates()
	case "databases":
		return s.databaseInventory()
	case "vulnerabilities":
		out := []vql.Row{}
		latest := map[string]bool{}
		for _, run := range s.Store.OperationRuns("") {
			if run.Kind != "software" || latest[run.ObjectID] || run.Status != "success" {
				continue
			}
			latest[run.ObjectID] = true
			values, _ := run.Result["findings"].([]any)
			for _, v := range values {
				if row, ok := v.(map[string]any); ok {
					copy := vql.Row{}
					for k, v := range row {
						copy[k] = v
					}
					copy["object_id"] = run.ObjectID
					copy["scanned_at"] = run.FinishedAt
					out = append(out, copy)
				}
			}
		}
		return out
	}
	return []vql.Row{}
}
func (s *Server) executeVQL(query string, minutes int) (vql.Result, error) {
	if minutes < 1 || minutes > 1440 {
		return vql.Result{}, fmt.Errorf("janela deve ser 1..1440 minutos")
	}
	source, _, err := vql.Source(query)
	if err != nil {
		return vql.Result{}, err
	}
	return vql.Execute(query, s.workspaceRows(source, minutes))
}
func (s *Server) workspaceRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/frontends/discovered", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{"frontends": s.frontendCandidates()})
	}))
	m.HandleFunc("GET /api/v1/databases/inventory", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{"databases": s.databaseInventory()})
	}))
	m.HandleFunc("GET /api/v1/workspace", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{"objects": s.Store.WorkspaceObjects(r.URL.Query().Get("kind")), "runs": s.Store.OperationRuns("")})
	}))
	m.HandleFunc("POST /api/v1/workspace", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		var o model.WorkspaceObject
		if !decode(w, r, &o) {
			return
		}
		if e := store.ValidateWorkspaceObject(o); e != nil {
			fail(w, 400, e)
			return
		}
		if e := s.Store.PutWorkspace(o); e != nil {
			fail(w, 500, e)
			return
		}
		write(w, 200, map[string]bool{"saved": true})
	}))
	m.HandleFunc("DELETE /api/v1/workspace/{id}", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if e := s.Store.DeleteWorkspace(r.PathValue("id")); e != nil {
			fail(w, 400, e)
			return
		}
		write(w, 200, map[string]bool{"deleted": true})
	}))
	m.HandleFunc("POST /api/v1/workspace/{id}/run", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		run, e := s.startOperation(r.Context(), r.PathValue("id"))
		if e != nil {
			fail(w, 400, e)
			return
		}
		write(w, 202, run)
	}))
	m.HandleFunc("POST /api/v1/vql", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Query   string `json:"query"`
			Minutes int    `json:"minutes"`
		}
		if !decode(w, r, &p) {
			return
		}
		if p.Minutes == 0 {
			p.Minutes = 60
		}
		result, e := s.executeVQL(p.Query, p.Minutes)
		if e != nil {
			fail(w, 400, e)
			return
		}
		write(w, 200, result)
	}))
	m.HandleFunc("POST /api/v1/software/import", s.adminAuth(s.importSoftware))
	m.HandleFunc("POST /api/v1/software/from-host/{id}", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range s.Store.Snapshot().Hosts {
			if h.ID == r.PathValue("id") {
				o := model.WorkspaceObject{ID: "software-" + operationID(), Name: h.Name + " · Pacotes do agente", Kind: "software", Packages: h.Packages}
				if e := s.Store.PutWorkspace(o); e != nil {
					fail(w, 400, e)
					return
				}
				write(w, 200, o)
				return
			}
		}
		http.NotFound(w, r)
	}))
}
func (s *Server) reserveOperation(id string) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := "workspace:" + id
	if s.running[key] || len(s.running) >= 4 {
		return nil, fmt.Errorf("operação já em execução ou limite de concorrência atingido")
	}
	s.running[key] = true
	return func() { s.mu.Lock(); delete(s.running, key); s.mu.Unlock() }, nil
}
func (s *Server) startOperation(ctx context.Context, id string) (model.OperationRun, error) {
	o, ok := s.Store.WorkspaceObject(id)
	if !ok || o.Kind == "notebook" {
		return model.OperationRun{}, fmt.Errorf("selecione workflow, banco ou inventário de software")
	}
	release, e := s.reserveOperation(id)
	if e != nil {
		return model.OperationRun{}, e
	}
	run := model.OperationRun{ID: operationID(), ObjectID: id, Kind: o.Kind, StartedAt: time.Now().UTC(), Status: "running", Result: map[string]any{}}
	if e := s.Store.SaveOperation(run); e != nil {
		release()
		return run, e
	}
	go func(run model.OperationRun) {
		defer release()
		job, cancel := context.WithTimeout(context.WithoutCancel(ctx), 90*time.Second)
		defer cancel()
		result, err := s.executeObject(job, o)
		run.Result = result
		if run.Result == nil {
			run.Result = map[string]any{}
		}
		run.FinishedAt = time.Now().UTC()
		run.Status = "success"
		if err != nil {
			run.Status = "error"
			run.Error = err.Error()
		}
		if e := s.Store.SaveOperation(run); e != nil {
			run.Status = "error"
			run.Error = "Não foi possível persistir o resultado completo"
			run.Result = map[string]any{}
			if err := s.Store.SaveOperation(run); err != nil {
				log.Print("workspace: falha ao persistir resultado da operação")
			}
		}
	}(run)
	return run, nil
}
func (s *Server) executeObject(ctx context.Context, o model.WorkspaceObject) (map[string]any, error) {
	switch o.Kind {
	case "database":
		return s.Runner.CheckDatabase(ctx, o)
	case "software":
		return s.Runner.ScanPackages(ctx, o.Packages)
	case "workflow":
		results := []any{}
		previousRows := -1
		for _, step := range o.Steps {
			if e := ctx.Err(); e != nil {
				return map[string]any{"steps": results}, e
			}
			row := map[string]any{"id": step.ID, "name": step.Name, "kind": step.Kind, "status": "success"}
			switch step.Kind {
			case "query":
				r, e := s.executeVQL(step.Query, 60)
				if e != nil {
					row["status"] = "error"
					row["error"] = e.Error()
					results = append(results, row)
					return map[string]any{"steps": results}, e
				}
				row["result"] = r
				previousRows = len(r.Rows)
			case "condition":
				if previousRows < 0 {
					return map[string]any{"steps": results}, fmt.Errorf("condição exige uma consulta anterior")
				}
				if previousRows < step.MinRows {
					row["status"] = "condition_not_met"
					results = append(results, row)
					return map[string]any{"steps": results, "skipped_remaining": true}, nil
				}
			case "database_check", "vulnerability_scan":
				target, ok := s.Store.WorkspaceObject(step.TargetID)
				expected := "database"
				if step.Kind == "vulnerability_scan" {
					expected = "software"
				}
				if !ok || target.Kind != expected {
					return map[string]any{"steps": results}, fmt.Errorf("perfil da etapa não existe ou tipo não confere")
				}
				child := model.OperationRun{ID: operationID(), ObjectID: target.ID, Kind: target.Kind, StartedAt: time.Now().UTC(), Status: "running", Result: map[string]any{}}
				if err := s.Store.SaveOperation(child); err != nil {
					return map[string]any{"steps": results}, fmt.Errorf("não foi possível iniciar a etapa")
				}
				value, e := s.executeObject(ctx, target)
				child.FinishedAt = time.Now().UTC()
				child.Status = "success"
				child.Result = value
				if e != nil {
					child.Status = "error"
					child.Error = e.Error()
				}
				if err := s.Store.SaveOperation(child); err != nil {
					return map[string]any{"steps": results}, fmt.Errorf("não foi possível persistir a etapa")
				}
				row["result"] = value
				if e != nil {
					row["status"] = "error"
					row["error"] = e.Error()
					results = append(results, row)
					return map[string]any{"steps": results}, e
				}
			default:
				return nil, fmt.Errorf("etapa não suportada")
			}
			results = append(results, row)
		}
		return map[string]any{"steps": results}, nil
	}
	return nil, fmt.Errorf("ação não suportada")
}
func (s *Server) pollWorkflows(ctx context.Context) {
	for _, o := range s.Store.WorkspaceObjects("workflow") {
		if !o.Enabled || o.IntervalSeconds < 60 {
			continue
		}
		last := o.UpdatedAt
		for _, r := range s.Store.OperationRuns(o.ID) {
			if r.StartedAt.After(last) {
				last = r.StartedAt
			}
		}
		if time.Since(last) < time.Duration(o.IntervalSeconds)*time.Second {
			continue
		}
		_, _ = s.startOperation(ctx, o.ID)
	}
}
func queryMinutes(r *http.Request) int {
	n, e := strconv.Atoi(r.URL.Query().Get("minutes"))
	if e != nil {
		return 60
	}
	return n
}
