package main

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectionPreservesApplicationAndPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, path, ct, body, csp string
		approved                  bool
		want                      string
		injected                  bool
	}{{"html", "/", "text/html", "<!DOCTYPE html><html><head><title>Test</title></head><body>App</body></html>", "", false, "injected", true}, {"json", "/api", "application/json", `{"key":"value"}`, "", false, "", false}, {"excluded", "/login", "text/html", "<head></head>", "", false, "excluded", false}, {"csp", "/", "text/html", "<head></head>", "script-src 'none'", false, "skipped-csp", false}, {"approved", "/", "text/html", "<head></head>", "script-src https://collector.test", true, "injected", true}, {"duplicate", "/", "text/html", `<head><script data-vetra-injected="true"></script></head>`, "", false, "already-present", false}} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.ct)
				w.Header().Set("ETag", "original")
				if tc.csp != "" {
					w.Header().Set("Content-Security-Policy", tc.csp)
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			defer upstream.Close()
			u, _ := url.Parse(upstream.URL)
			proxy := injector(u, "https://collector.test/rum/vetra.js?app=shop", []string{"/login"}, tc.approved)
			w := httptest.NewRecorder()
			proxy.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
			if w.Code != 200 || w.Header().Get("X-Vetra-Injection") != tc.want {
				t.Fatal(w.Code, w.Header())
			}
			if tc.injected {
				if !strings.Contains(w.Body.String(), "https://collector.test/rum/vetra.js?app=shop") || w.Header().Get("ETag") != "" || w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("missing injection/cache fix")
				}
			} else if w.Body.String() != tc.body {
				t.Fatal("unrelated response changed")
			}
			if w.Header().Get("Content-Security-Policy") != tc.csp {
				t.Fatal("CSP was weakened")
			}
		})
	}
}

func TestGzipHTMLAndRealHeadToken(t *testing.T) {
	html := `<!doctype html><!-- <head>do not inject here</head> --><html><head><title>App</title></head><body>App</body></html>`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		io.WriteString(gz, html)
		gz.Close()
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	p := injector(u, "https://collector.test/rum/vetra.js?app=portal", nil, false)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	gz, e := gzip.NewReader(w.Body)
	if e != nil {
		t.Fatal(e)
	}
	b, e := io.ReadAll(gz)
	if e != nil || !strings.Contains(string(b), `<html><head><script data-vetra-injected=`) || !strings.Contains(string(b), `<!-- <head>do not inject here</head> -->`) {
		t.Fatalf("%s %v", b, e)
	}
}
func TestRoutesIsolationBalanceReloadAndBypass(t *testing.T) {
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<html><head></head><body>A</body></html>")
	}))
	defer a.Close()
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<html><head></head><body>B</body></html>")
	}))
	defer b.Close()
	file := filepath.Join(t.TempDir(), "routes.json")
	config := routeConfig{Routes: []applicationRoute{{ID: "portal", Host: "portal.test", PathPrefix: "/", Upstreams: []string{a.URL, b.URL}, ScriptURL: "https://collector.test/rum/vetra.js?app=portal"}}}
	raw, _ := json.Marshal(config)
	os.WriteFile(file, raw, 0600)
	m, e := newRouteManager(file)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		m.ServeHTTP(w, httptest.NewRequest("GET", "http://portal.test/", nil))
		if w.Header().Get("X-Vetra-Injection") != "injected" || !strings.Contains(w.Body.String(), []string{"<body>A", "<body>B"}[i]) {
			t.Fatal(w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	m.ServeHTTP(w, httptest.NewRequest("GET", "http://other.test/", nil))
	if w.Code != 404 {
		t.Fatal("host isolation")
	}
	os.WriteFile(file, []byte(`{"routes":[]}`), 0600)
	if m.reload() == nil {
		t.Fatal("accepted invalid reload")
	}
	w = httptest.NewRecorder()
	m.ServeHTTP(w, httptest.NewRequest("GET", "http://portal.test/", nil))
	if w.Code != 200 {
		t.Fatal("lost last valid configuration")
	}
	disabled := false
	config.Routes[0].InjectionEnabled = &disabled
	raw, _ = json.Marshal(config)
	os.WriteFile(file, raw, 0600)
	if e = m.reload(); e != nil {
		t.Fatal(e)
	}
	w = httptest.NewRecorder()
	m.ServeHTTP(w, httptest.NewRequest("GET", "http://portal.test/", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "data-vetra-injected") {
		t.Fatal("bypass broke routing")
	}
}

func TestProxyCircuitAndForwardedHeaders(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Forwarded") != "" || r.Header.Get("X-Forwarded-Host") != "portal.test" || r.Header.Get("X-Forwarded-Proto") != "http" || strings.Contains(r.Header.Get("X-Forwarded-For"), "spoof") {
			t.Error("spoofed forwarding headers accepted")
		}
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<head></head>")
	}))
	defer backend.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	badURL := bad.URL
	bad.Close()
	raw, _ := json.Marshal(routeConfig{Routes: []applicationRoute{{ID: "portal", Host: "portal.test", PathPrefix: "/", Upstreams: []string{badURL, backend.URL}, ScriptURL: "https://collector.test/rum/vetra.js?app=portal"}}})
	set, e := compileRoutes(raw)
	if e != nil {
		t.Fatal(e)
	}
	m := &routeManager{}
	m.current.Store(set)
	for i, want := range []int{502, 200, 200} {
		r := httptest.NewRequest("GET", "http://portal.test/", nil)
		r.Header.Set("Forwarded", "spoof")
		r.Header.Set("X-Forwarded-For", "spoof")
		r.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)
		if w.Code != want {
			t.Errorf("request %d: %d", i, w.Code)
		}
	}
}
