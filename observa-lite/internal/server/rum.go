package server

import (
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

type rumBudget struct {
	At    time.Time
	Count int
}

func (s *Server) rumRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/rum/apps", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{"applications": s.Store.RUMApplications()})
	}))
	m.HandleFunc("POST /api/v1/rum/apps", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		var a model.RUMApplication
		if !decode(w, r, &a) {
			return
		}
		if err := store.ValidateRUMApplication(a); err != nil {
			fail(w, 400, err)
			return
		}
		if err := s.Store.PutRUMApplication(a); err != nil {
			fail(w, 500, err)
			return
		}
		write(w, 200, map[string]bool{"saved": true})
	}))
	m.HandleFunc("DELETE /api/v1/rum/apps/{id}", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if err := s.Store.DeleteRUMApplication(r.PathValue("id")); err != nil {
			fail(w, 400, err)
			return
		}
		write(w, 200, map[string]bool{"deleted": true})
	}))
	m.HandleFunc("GET /api/v1/rum/analysis", s.adminAuth(s.rumAnalysis))
	m.HandleFunc("GET /api/v1/rum/apps/{id}/sessions/{session}/replay", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{"chunks": s.Store.RUMReplay(r.PathValue("id"), r.PathValue("session"))})
	}))
	m.HandleFunc("DELETE /api/v1/rum/apps/{id}/sessions/{session}", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if err := s.Store.RUMPurgeSession(r.PathValue("id"), r.PathValue("session")); err != nil {
			fail(w, 500, err)
			return
		}
		write(w, 200, map[string]bool{"deleted": true})
	}))
	m.HandleFunc("GET /rum/vetra.js", func(w http.ResponseWriter, r *http.Request) {
		b, e := public.ReadFile("public/app/rum/vetra.js")
		if e != nil {
			http.Error(w, "Build the browser SDK with npm run build", 503)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(b)
	})
	m.HandleFunc("GET /rum/config/{id}", s.rumConfig)
	m.HandleFunc("OPTIONS /rum/beacon/{id}", s.rumOptions)
	m.HandleFunc("POST /rum/beacon/{id}", s.rumBeacon)
}
func (s *Server) rumOrigin(w http.ResponseWriter, r *http.Request) (model.RUMApplication, bool) {
	a, ok := s.Store.RUMApplication(r.PathValue("id"))
	if !ok || !a.Enabled {
		http.Error(w, "frontend disabled", 404)
		return a, false
	}
	origin := r.Header.Get("Origin")
	if origin == "" && r.Method == http.MethodGet {
		if u, e := url.Parse(r.Referer()); e == nil && u.Host != "" {
			origin = u.Scheme + "://" + u.Host
		}
	}
	allowed := false
	for _, v := range a.Origins {
		if v == origin {
			allowed = true
			break
		}
	}
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !allowed {
		http.Error(w, "origin not allowed", 403)
		return a, false
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	return a, true
}
func (s *Server) rumConfig(w http.ResponseWriter, r *http.Request) {
	a, ok := s.rumOrigin(w, r)
	if !ok {
		return
	}
	write(w, 200, a)
}
func (s *Server) rumOptions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.rumOrigin(w, r); !ok {
		return
	}
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Max-Age", "300")
	w.WriteHeader(204)
}
func (s *Server) rumBeacon(w http.ResponseWriter, r *http.Request) {
	a, ok := s.rumOrigin(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	if s.rumLimits == nil {
		s.rumLimits = map[string]rumBudget{}
	}
	b := s.rumLimits[a.ID]
	if time.Since(b.At) > time.Minute {
		b = rumBudget{At: time.Now()}
	}
	b.Count++
	s.rumLimits[a.ID] = b
	s.mu.Unlock()
	if b.Count > 600 {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "RUM rate limit", 429)
		return
	}
	var payload struct {
		Consent bool                   `json:"consent"`
		Events  []model.RUMEvent       `json:"events"`
		Replay  []model.RUMReplayChunk `json:"replay"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&payload); err != nil {
		http.Error(w, "invalid RUM payload", 400)
		return
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		http.Error(w, "multiple payloads", 400)
		return
	}
	if a.RequireConsent && !payload.Consent {
		http.Error(w, "consent required", 403)
		return
	}
	if err := s.Store.IngestRUM(a.ID, payload.Events, payload.Replay); err != nil {
		fail(w, 400, err)
		return
	}
	w.WriteHeader(202)
}

type rumSession struct {
	ID      string    `json:"id"`
	AppID   string    `json:"app_id"`
	Started time.Time `json:"started"`
	Last    time.Time `json:"last"`
	Pages   int       `json:"pages"`
	Errors  int       `json:"errors"`
	Browser string    `json:"browser"`
	OS      string    `json:"os"`
	Device  string    `json:"device"`
	User    string    `json:"user"`
}

func percentile75(v []float64) *float64 {
	if len(v) == 0 {
		return nil
	}
	sort.Float64s(v)
	index := (3*len(v)+3)/4 - 1
	if index < 0 {
		index = 0
	}
	n := v[index]
	return &n
}
func (s *Server) rumAnalysis(w http.ResponseWriter, r *http.Request) {
	minutes := 60
	if n, e := strconv.Atoi(r.URL.Query().Get("minutes")); e == nil && n >= 1 && n <= 1440 {
		minutes = n
	}
	id := r.URL.Query().Get("app_id")
	a, hasApp := s.Store.RUMApplication(id)
	threshold := 1000.0
	if hasApp {
		threshold = a.ApdexT
	}
	events := s.Store.RUMEvents(id, time.Now().Add(-time.Duration(minutes)*time.Minute))
	browser, os, device := r.URL.Query().Get("browser"), r.URL.Query().Get("os"), r.URL.Query().Get("device")
	filtered := []model.RUMEvent{}
	for _, e := range events {
		if (browser == "" || e.Browser == browser) && (os == "" || e.OS == os) && (device == "" || e.Device == device) {
			filtered = append(filtered, e)
		}
	}
	sessions := map[string]*rumSession{}
	vitals := map[string][]float64{}
	pages, requests, errors := 0, 0, 0
	apdexPages := 0
	errorsByPage := map[string]bool{}
	for _, e := range filtered {
		if e.Kind == "error" {
			errorsByPage[e.AppID+":"+e.SessionID+":"+e.PageID] = true
		}
	}
	satisfied, tolerated, frustrated := 0, 0, 0
	for _, e := range filtered {
		k := e.AppID + ":" + e.SessionID
		v := sessions[k]
		if v == nil {
			v = &rumSession{ID: e.SessionID, AppID: e.AppID, Started: e.At, Last: e.At, Browser: e.Browser, OS: e.OS, Device: e.Device}
			sessions[k] = v
		}
		if e.At.Before(v.Started) {
			v.Started = e.At
		}
		if e.At.After(v.Last) {
			v.Last = e.At
		}
		if e.UserID != "" {
			v.User = e.UserID
		}
		switch e.Kind {
		case "navigation":
			pages++
			v.Pages++
		case "page_view":
			apdexPages++
			pages++
			v.Pages++
			t := threshold
			if id == "" {
				if cfg, ok := s.Store.RUMApplication(e.AppID); ok {
					t = cfg.ApdexT
				}
			}
			if e.Error || errorsByPage[e.AppID+":"+e.SessionID+":"+e.PageID] || e.Duration > 4*t {
				frustrated++
			} else if e.Duration <= t {
				satisfied++
			} else {
				tolerated++
			}
		case "request":
			requests++
			if e.Error || e.Status >= 400 {
				errors++
				v.Errors++
			}
		case "error":
			errors++
			v.Errors++
		case "vital":
			vitals[e.Name] = append(vitals[e.Name], e.Value)
		}
	}
	var apdex *float64
	if apdexPages > 0 {
		n := (float64(satisfied) + float64(tolerated)/2) / float64(apdexPages)
		apdex = &n
	}
	metrics := map[string]*float64{}
	for _, k := range []string{"LCP", "INP", "CLS", "FCP", "TTFB"} {
		metrics[k] = percentile75(vitals[k])
	}
	list := []rumSession{}
	users := map[string]bool{}
	for _, v := range sessions {
		list = append(list, *v)
		if v.User != "" {
			users[v.User] = true
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Last.After(list[j].Last) })
	if len(list) > 500 {
		list = list[:500]
	}
	// Raw client telemetry is bounded and intentionally contains no request bodies or stack traces.
	recent := filtered
	if len(recent) > 1000 {
		recent = recent[len(recent)-1000:]
	}
	write(w, 200, map[string]any{"minutes": minutes, "pages": pages, "requests": requests, "errors": errors, "sessions_total": len(sessions), "identified_users": len(users), "apdex": apdex, "apdex_t_ms": threshold, "apdex_samples": apdexPages, "apdex_counts": map[string]int{"satisfied": satisfied, "tolerated": tolerated, "frustrated": frustrated}, "vitals_p75": metrics, "sessions": list, "events": recent, "retention_hours": 24, "source": "browser-self-reported", "native_mobile_sdk": false, "note": fmt.Sprintf("Apdex considera %d carregamentos iniciais; navegações SPA não fabricam tempos de renderização.", apdexPages)})
}
