package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
