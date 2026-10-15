package server

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/collectors"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

//go:embed public/*
var public embed.FS

type Server struct {
	Store                  *store.Store
	AdminToken, AgentToken string
	Runner                 collectors.Runner
	mu                     sync.Mutex
	running                map[string]bool
	rumLimits              map[string]rumBudget
}

func New(st *store.Store, admin, agent string, allowPrivate bool) *Server {
	return &Server{Store: st, AdminToken: admin, AgentToken: agent, Runner: collectors.Runner{AllowPrivate: allowPrivate}, running: map[string]bool{}}
}
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	s.userRoutes(m)
	s.downloadRoutes(m)
	m.HandleFunc("POST /api/v1/storage/export", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorizeUser(w, r, true) {
			return
		}
		b, e := s.Store.ExportJSON()
		if e != nil {
			fail(w, 500, e)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="vetra-state-export.json"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Write(b)
	})
	m.HandleFunc("GET /api/v1/storage", s.adminAuth(func(w http.ResponseWriter, r *http.Request) { write(w, 200, s.Store.StorageInfo()) }))
	s.rumRoutes(m)
	s.workspaceRoutes(m)
	m.HandleFunc("GET /api/v1/integrations", s.adminAuth(s.integrationList))
	m.HandleFunc("POST /api/v1/integrations", s.adminAuth(s.integrationSave))
	m.HandleFunc("DELETE /api/v1/integrations/{id}", s.adminAuth(s.integrationDelete))
	m.HandleFunc("POST /api/v1/integrations/{id}/collect", s.adminAuth(s.integrationCollect))
	m.HandleFunc("PATCH /api/v1/hosts/{id}", s.adminAuth(s.editInventory))
	m.HandleFunc("GET /api/v1/topology", s.adminAuth(s.topologyViews))
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]bool{"ok": true}) })
	m.HandleFunc("GET /", s.serveUI)
	m.HandleFunc("POST /api/v1/ingest", s.agentAuth(s.ingest))
	m.HandleFunc("POST /v1/traces", s.agentAuth(s.ingestTraces))
	m.HandleFunc("POST /v1/logs", s.agentAuth(s.ingestLogs))
	m.HandleFunc("POST /v1/metrics", s.agentAuth(s.ingestMetrics))
	m.HandleFunc("GET /api/v1/telemetry/logs", s.adminAuth(s.telemetryLogs))
	m.HandleFunc("GET /api/v1/telemetry/metrics", s.adminAuth(s.telemetryMetrics))
	m.HandleFunc("GET /api/v1/apm/analysis", s.adminAuth(s.apmAnalysis))
	m.HandleFunc("GET /api/v1/slos", s.adminAuth(s.sloEvaluations))
	m.HandleFunc("POST /api/v1/slos", s.adminAuth(s.saveSLO))
	m.HandleFunc("DELETE /api/v1/slos/{id}", s.adminAuth(s.deleteSLO))
	m.HandleFunc("GET /api/v1/apm/rules", s.adminAuth(s.apmRules))
	m.HandleFunc("POST /api/v1/apm/rules", s.adminAuth(s.saveAPMRule))
	m.HandleFunc("DELETE /api/v1/apm/rules/{id}", s.adminAuth(s.deleteAPMRule))
	m.HandleFunc("POST /api/v1/apm/rules/evaluate", s.adminAuth(s.evaluateAPMRules))
	m.HandleFunc("GET /api/v1/apm/problems", s.adminAuth(s.apmProblems))
	m.HandleFunc("GET /api/v1/apm/services", s.adminAuth(s.apmServices))
	m.HandleFunc("GET /api/v1/apm/timeseries", s.adminAuth(s.apmTimeseries))
	m.HandleFunc("GET /api/v1/apm/traces", s.adminAuth(s.apmTraces))
	m.HandleFunc("GET /api/v1/apm/traces/{id}", s.adminAuth(s.apmTrace))
	m.HandleFunc("GET /api/v1/apm/map", s.adminAuth(s.apmMap))
	m.HandleFunc("GET /api/v1/agent/connectioninfo", s.agentAuth(s.agentConnectionInfo))
	m.HandleFunc("GET /api/v1/gateways", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{"gateways": s.Store.Gateways()})
	}))
	m.HandleFunc("POST /api/v1/gateways", s.adminAuth(s.registerGateway))
	m.HandleFunc("DELETE /api/v1/gateways/{id}", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if err := s.Store.DeleteGateway(r.PathValue("id")); err != nil {
			fail(w, 500, err)
			return
		}
		write(w, 200, map[string]bool{"deleted": true})
	}))
	m.HandleFunc("GET /api/v1/snapshot", s.adminAuth(func(w http.ResponseWriter, r *http.Request) { write(w, 200, s.Store.Snapshot()) }))
	m.HandleFunc("GET /api/v1/extensions", s.adminAuth(s.extensions))
	m.HandleFunc("GET /api/v1/capabilities", s.adminAuth(s.capabilities))
	m.HandleFunc("POST /api/v1/templates", s.adminAuth(s.upsertTemplate))
	m.HandleFunc("DELETE /api/v1/templates/{id}", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if e := s.Store.DeleteTemplate(r.PathValue("id")); e != nil {
			fail(w, 500, e)
			return
		}
		write(w, 200, map[string]bool{"ok": true})
	}))
	m.HandleFunc("POST /api/v1/datasources", s.adminAuth(s.upsertSource))
	m.HandleFunc("POST /api/v1/dashboards", s.adminAuth(s.upsertDashboard))
	m.HandleFunc("DELETE /api/v1/dashboards/{id}", s.adminAuth(s.deleteDashboard))
	m.HandleFunc("GET /api/v1/query", s.adminAuth(s.query))
	m.HandleFunc("GET /api/v1/logs", s.adminAuth(s.logs))
	m.HandleFunc("POST /mcp", s.adminAuth(s.mcp))
	return m
}

// serveUI embeds the compiled Lovable React SPA when available.
// During source-only development, the original HTML page remains the fallback.
func (s *Server) serveUI(w http.ResponseWriter, r *http.Request) {
	requested := path.Clean("/" + r.URL.Path)
	index := requested == "/" || requested == "/index.html" || !strings.Contains(path.Base(requested), ".")
	name := "public/app" + requested
	if index {
		name = "public/app/index.html"
	}
	body, err := public.ReadFile(name)
	modern := err == nil
	if err != nil {
		if !index {
			http.NotFound(w, r)
			return
		}
		body, err = public.ReadFile("public/index.html")
		if err != nil {
			http.Error(w, "frontend indisponível", http.StatusServiceUnavailable)
			return
		}
		name = "public/index.html"
	}
	typ := mime.TypeByExtension(path.Ext(name))
	if typ == "" {
		typ = "application/octet-stream"
	}
	if path.Ext(name) == ".html" {
		typ = "text/html; charset=utf-8"
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	if strings.HasPrefix(name, "public/app/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}
	if modern {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self'; frame-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'; object-src 'none'")
	} else {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	}
	w.WriteHeader(200)
	_, _ = w.Write(body)
}
func (s *Server) checkToken(r *http.Request, expected string) bool {
	value := r.Header.Get("X-API-Key")
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Authorization")), "bearer ") {
		value = r.Header.Get("Authorization")[7:]
	}
	return len(expected) >= 24 && subtle.ConstantTimeCompare([]byte(value), []byte(expected)) == 1
}
func (s *Server) auth(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.checkToken(r, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
func (s *Server) adminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.authorizeUser(w, r, false) {
			next(w, r)
		}
	}
}
func (s *Server) agentAuth(next http.HandlerFunc) http.HandlerFunc { return s.auth(s.AgentToken, next) }
func decode(w http.ResponseWriter, r *http.Request, to interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if e := dec.Decode(to); e != nil {
		http.Error(w, "invalid JSON: "+e.Error(), 400)
		return false
	}
	var extra interface{}
	if e := dec.Decode(&extra); e != io.EOF {
		http.Error(w, "unexpected trailing JSON", 400)
		return false
	}
	return true
}
func write(w http.ResponseWriter, code int, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(obj)
}
func fail(w http.ResponseWriter, code int, e error) { http.Error(w, e.Error(), code) }
func (s *Server) ingest(w http.ResponseWriter, r *http.Request) {
	var v model.Discovery
	if !decode(w, r, &v) {
		return
	}
	if e := s.Store.Ingest(v); e != nil {
		fail(w, 400, e)
		return
	}
	write(w, 202, map[string]bool{"accepted": true})
}
func (s *Server) upsertTemplate(w http.ResponseWriter, r *http.Request) {
	var v model.Template
	if !decode(w, r, &v) {
		return
	}
	if v.Kind != "http" && v.Kind != "tcp" {
		fail(w, 400, errors.New("available collector kinds: http, tcp"))
		return
	}
	if len(v.Target) > 1024 || strings.TrimSpace(v.Name) == "" {
		fail(w, 400, errors.New("name and target required; max target 1024"))
		return
	}
	if e := s.Store.PutTemplate(v); e != nil {
		fail(w, 500, e)
		return
	}
	write(w, 200, map[string]bool{"saved": true})
}
func (s *Server) upsertSource(w http.ResponseWriter, r *http.Request) {
	var v model.DataSource
	if !decode(w, r, &v) {
		return
	}
	if v.Kind != "prometheus" && v.Kind != "json" && v.Kind != "loki" || len(v.URL) > 2048 || v.Name == "" {
		fail(w, 400, errors.New("datasource must be prometheus, json or loki with name/URL"))
		return
	}
	if strings.ContainsAny(v.SecretEnv, " /\\:\n\t") || len(v.SecretEnv) > 100 {
		fail(w, 400, errors.New("invalid secret environment key"))
		return
	}
	if e := s.Store.PutSource(v); e != nil {
		fail(w, 500, e)
		return
	}
	write(w, 200, map[string]bool{"saved": true})
}
func (s *Server) upsertDashboard(w http.ResponseWriter, r *http.Request) {
	var v model.Dashboard
	if !decode(w, r, &v) {
		return
	}
	if strings.TrimSpace(v.Title) == "" || len(v.Panels) > 50 {
		fail(w, 400, errors.New("title required, max 50 panels"))
		return
	}
	for _, p := range v.Panels {
		if len(p.HTML)+len(p.CSS)+len(p.JS) > 100000 {
			fail(w, 400, errors.New("panel content too large"))
			return
		}
	}
	if e := s.Store.PutDashboard(v); e != nil {
		if errors.Is(e, store.ErrDashboardValidation) || errors.Is(e, store.ErrDashboardLimit) {
			fail(w, 400, e)
			return
		}
		fail(w, 500, e)
		return
	}
	write(w, 200, map[string]bool{"saved": true})
}
func (s *Server) query(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("source")
	expr := r.URL.Query().Get("expr")
	if len(expr) > 2000 {
		fail(w, 400, errors.New("query too long"))
		return
	}
	v, ok := s.Store.Source(id)
	if !ok {
		fail(w, 404, errors.New("source not found"))
		return
	}
	result, e := s.Runner.Query(r.Context(), v, expr)
	if e != nil {
		fail(w, 502, e)
		return
	}
	write(w, 200, result)
}
func (s *Server) Poll(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		s.pollOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Server) pollOnce(ctx context.Context) {
	if e := s.Store.PruneExpired(time.Now()); e != nil {
		log.Print("storage retention maintenance failed")
	}
	s.pollIntegrations(ctx)
	s.pollWorkflows(ctx)
	for _, t := range s.Store.Templates() {
		if !t.Enabled || time.Since(t.LastRun) < time.Duration(t.IntervalSeconds)*time.Second {
			continue
		}
		s.mu.Lock()
		if s.running[t.ID] {
			s.mu.Unlock()
			continue
		}
		if len(s.running) >= 4 {
			s.mu.Unlock()
			break
		}
		s.running[t.ID] = true
		s.mu.Unlock()
		go func(t model.Template) {
			defer func() { s.mu.Lock(); delete(s.running, t.ID); s.mu.Unlock() }()
			ok, value, msg := s.Runner.Check(ctx, t)
			if err := s.Store.TemplateResult(t.ID, time.Now().UTC(), ok, value, msg); err != nil {
				log.Printf("template persistence: %v", err)
			}
		}(t)
	}
}
func (s *Server) Listen(ctx context.Context, addr string) error {
	if len(s.AdminToken) < 24 || len(s.AgentToken) < 24 {
		return fmt.Errorf("OBSERVA_ADMIN_TOKEN and OBSERVA_AGENT_TOKEN must be at least 24 characters")
	}
	srv := &http.Server{Addr: addr, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 1 << 16}
	go s.Poll(ctx)
	go s.PollAPMRules(ctx)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	log.Printf("observa server listening on %s (HTTPS termination required for remote access)", addr)
	var e error
	certFile, keyFile := os.Getenv("OBSERVA_TLS_CERT_FILE"), os.Getenv("OBSERVA_TLS_KEY_FILE")
	if (certFile == "") != (keyFile == "") {
		return fmt.Errorf("TLS certificate and key must both be configured")
	}
	if certFile != "" {
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		e = srv.ListenAndServeTLS(certFile, keyFile)
	} else {
		e = srv.ListenAndServe()
	}
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
func AllowPrivateFromEnv() bool { return os.Getenv("OBSERVA_ALLOW_PRIVATE_TARGETS") == "true" }

func (s *Server) registerGateway(w http.ResponseWriter, r *http.Request) {
	var g model.GatewayEndpoint
	if !decode(w, r, &g) {
		return
	}
	if err := s.Store.UpsertGateway(g); err != nil {
		fail(w, 400, err)
		return
	}
	write(w, 200, map[string]bool{"saved": true})
}
func (s *Server) agentConnectionInfo(w http.ResponseWriter, r *http.Request) {
	zone := r.URL.Query().Get("network_zone")
	if len(zone) > 120 {
		http.Error(w, "invalid network zone", 400)
		return
	}
	allowFallback := r.URL.Query().Get("allow_default_fallback") == "true"
	info := s.Store.ConnectionInfo(zone, allowFallback)
	if len(info.Endpoints) == 0 {
		write(w, 503, map[string]any{"error": "no eligible gateway endpoints", "network_zone": info.NetworkZone})
		return
	}
	write(w, 200, info)
}
