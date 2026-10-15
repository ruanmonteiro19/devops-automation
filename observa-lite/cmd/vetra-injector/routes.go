package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

type routeConfig struct {
	Routes []applicationRoute `json:"routes"`
}
type applicationRoute struct {
	ID               string   `json:"id"`
	Host             string   `json:"host"`
	PathPrefix       string   `json:"path_prefix"`
	Upstreams        []string `json:"upstreams"`
	ScriptURL        string   `json:"script_url"`
	ExcludedPaths    []string `json:"excluded_paths"`
	CSPApproved      bool     `json:"csp_approved"`
	Disabled         bool     `json:"disabled"`
	InjectionEnabled *bool    `json:"injection_enabled,omitempty"`
}
type backend struct {
	handler     http.Handler
	failedUntil atomic.Int64
}
type routePool struct {
	config   applicationRoute
	backends []*backend
	next     atomic.Uint64
}
type routeSet struct{ pools []*routePool }
type routeManager struct {
	file    string
	current atomic.Pointer[routeSet]
	digest  [32]byte
}

func validURL(raw string, script bool) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	if script {
		return u.Query().Get("app") != ""
	}
	return u.RawQuery == ""
}
func compileRoutes(raw []byte) (*routeSet, error) {
	var cfg routeConfig
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&cfg); e != nil {
		return nil, e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("invalid trailing JSON")
	}
	if len(cfg.Routes) < 1 || len(cfg.Routes) > 100 {
		return nil, fmt.Errorf("configure 1..100 routes")
	}
	seen := map[string]bool{}
	ids := map[string]bool{}
	set := &routeSet{}
	for _, route := range cfg.Routes {
		route.Host = strings.ToLower(route.Host)
		if route.ID == "" || len(route.ID) > 96 || ids[route.ID] || len(route.Host) > 253 || route.Host == "" || strings.ContainsAny(route.Host, "/ \\?#@:") || !strings.HasPrefix(route.PathPrefix, "/") || strings.ContainsAny(route.PathPrefix, "?#\r\n") || len(route.PathPrefix) > 512 || !validURL(route.ScriptURL, true) || len(route.Upstreams) < 1 || len(route.Upstreams) > 16 {
			return nil, fmt.Errorf("invalid route %q", route.ID)
		}
		key := route.Host + "|" + route.PathPrefix
		if seen[key] {
			return nil, fmt.Errorf("duplicate route")
		}
		seen[key] = true
		ids[route.ID] = true
		if route.Disabled {
			continue
		}
		if route.ExcludedPaths == nil {
			route.ExcludedPaths = []string{"/login", "/auth", "/checkout", "/payment"}
		}
		if len(route.ExcludedPaths) > 100 {
			return nil, fmt.Errorf("too many excluded paths")
		}
		for _, p := range route.ExcludedPaths {
			if !strings.HasPrefix(p, "/") || len(p) > 512 {
				return nil, fmt.Errorf("invalid exclusion")
			}
		}
		pool := &routePool{config: route}
		for _, raw := range route.Upstreams {
			if !validURL(raw, false) {
				return nil, fmt.Errorf("invalid upstream")
			}
			u, _ := url.Parse(raw)
			b := &backend{}
			script := route.ScriptURL
			if route.InjectionEnabled != nil && !*route.InjectionEnabled {
				script = ""
			}
			proxy := injector(u, script, route.ExcludedPaths, route.CSPApproved).(*httputil.ReverseProxy)
			proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
				b.failedUntil.Store(time.Now().Add(5 * time.Second).UnixNano())
				http.Error(w, "Application upstream unavailable", 502)
			}
			b.handler = proxy
			pool.backends = append(pool.backends, b)
		}
		set.pools = append(set.pools, pool)
	}
	sort.SliceStable(set.pools, func(i, j int) bool {
		if set.pools[i].config.Host == "*" && set.pools[j].config.Host != "*" {
			return false
		}
		if set.pools[j].config.Host == "*" && set.pools[i].config.Host != "*" {
			return true
		}
		return len(set.pools[i].config.PathPrefix) > len(set.pools[j].config.PathPrefix)
	})
	return set, nil
}
func newRouteManager(file string) (*routeManager, error) {
	m := &routeManager{file: file}
	if e := m.reload(); e != nil {
		return nil, e
	}
	return m, nil
}
func (m *routeManager) reload() error {
	f, e := os.Open(m.file)
	if e != nil {
		return e
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if e != nil || len(raw) > 1<<20 {
		return fmt.Errorf("configuration exceeds limit")
	}
	digest := sha256.Sum256(raw)
	if digest == m.digest {
		return nil
	}
	set, e := compileRoutes(raw)
	if e != nil {
		return e
	}
	m.current.Store(set)
	m.digest = digest
	return nil
}
func (m *routeManager) watch(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if e := m.reload(); e != nil {
				log.Print("Vetra: invalid route update; keeping last valid configuration")
			}
		}
	}
}
func (m *routeManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/__vetra/health" && r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"configured":true}`)
		return
	}
	host := r.Host
	if h, _, e := net.SplitHostPort(host); e == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	set := m.current.Load()
	for _, pool := range set.pools {
		cfg := pool.config
		if cfg.Host != "*" && cfg.Host != host {
			continue
		}
		prefix := strings.TrimSuffix(cfg.PathPrefix, "/")
		if prefix != "" && r.URL.Path != prefix && !strings.HasPrefix(r.URL.Path, prefix+"/") {
			continue
		}
		start := int(pool.next.Add(1)-1) % len(pool.backends)
		now := time.Now().UnixNano()
		for i := 0; i < len(pool.backends); i++ {
			b := pool.backends[(start+i)%len(pool.backends)]
			if b.failedUntil.Load() > now {
				continue
			}
			b.handler.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Retry-After", "5")
		http.Error(w, "Application upstreams unavailable", 503)
		return
	}
	http.NotFound(w, r)
}
func bounded(next http.Handler, max int) http.Handler {
	slots := make(chan struct{}, max)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Vetra proxy busy", 503)
		}
	})
}
