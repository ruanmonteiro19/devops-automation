// Vetra injector is an opt-in edge proxy. It does not instrument native mobile apps.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var headTag = regexp.MustCompile(`(?i)<head(?:\s[^>]*)?>`)

func injector(upstream *url.URL, script string, excluded []string, cspApproved bool) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	baseDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		baseDirector(r)
		r.Host = upstream.Host
		r.Header.Del("Accept-Encoding")
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			r.Header.Del("If-None-Match")
			r.Header.Del("If-Modified-Since")
		}
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		http.Error(w, "Application upstream unavailable", 502)
	}
	proxy.ModifyResponse = func(r *http.Response) error {
		if r.Request.Method == "HEAD" || r.StatusCode != 200 || !strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "text/html") {
			return nil
		}
		for _, p := range excluded {
			if p != "" && strings.HasPrefix(r.Request.URL.Path, p) {
				r.Header.Set("X-Vetra-Injection", "excluded")
				return nil
			}
		}
		if r.Header.Get("Content-Encoding") != "" && r.Header.Get("Content-Encoding") != "identity" {
			r.Header.Set("X-Vetra-Injection", "skipped-encoding")
			return nil
		}
		if r.Header.Get("Content-Security-Policy") != "" && !cspApproved {
			r.Header.Set("X-Vetra-Injection", "skipped-csp")
			return nil
		}
		b, e := io.ReadAll(io.LimitReader(r.Body, 8<<20+1))
		if e != nil {
			return e
		}
		if len(b) > 8<<20 {
			r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(b), r.Body))
			r.Header.Set("X-Vetra-Injection", "skipped-size")
			return nil
		}
		_ = r.Body.Close()
		result := b
		status := "skipped-html"
		lower := strings.ToLower(string(b))
		loc := headTag.FindIndex(b)
		if strings.Contains(lower, "content-security-policy") && !cspApproved {
			status = "skipped-meta-csp"
		} else if strings.Contains(lower, "data-vetra-injected") || strings.Contains(lower, "/rum/vetra.js") {
			status = "already-present"
		} else if loc != nil {
			tag := []byte(`<script data-vetra-injected="true" src="` + html.EscapeString(script) + `" crossorigin="anonymous"></script>`)
			result = make([]byte, 0, len(b)+len(tag))
			result = append(result, b[:loc[1]]...)
			result = append(result, tag...)
			result = append(result, b[loc[1]:]...)
			status = "injected"
		}
		r.Body = io.NopCloser(bytes.NewReader(result))
		r.ContentLength = int64(len(result))
		r.Header.Set("Content-Length", strconv.Itoa(len(result)))
		r.Header.Set("X-Vetra-Injection", status)
		if status == "injected" {
			for _, k := range []string{"ETag", "Last-Modified", "Content-MD5", "Digest", "Content-Digest"} {
				r.Header.Del(k)
			}
			r.Header.Set("Cache-Control", "no-store")
		}
		return nil
	}
	return proxy
}
func main() {
	listen := flag.String("listen", ":8080", "HTTP listener; terminate public TLS at the ingress")
	raw := flag.String("upstream", "", "fixed upstream application URL")
	script := flag.String("script-url", "", "absolute Vetra browser SDK URL with ?app=frontend-id")
	exclude := flag.String("exclude-paths", "/login,/auth,/checkout,/payment", "comma-separated URL prefixes excluded from injection")
	csp := flag.Bool("csp-approved", false, "operator confirms existing CSP permits script origin and collector; CSP is preserved")
	flag.Parse()
	u, e := url.Parse(*raw)
	s, se := url.Parse(*script)
	if e != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || se != nil || s.Hostname() == "" || s.User != nil || (s.Scheme != "http" && s.Scheme != "https") || s.Query().Get("app") == "" {
		log.Fatal("valid fixed upstream and public script-url with app ID are required")
	}
	srv := &http.Server{Addr: *listen, Handler: injector(u, *script, strings.Split(*exclude, ","), *csp), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32768}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	log.Printf("Vetra HTML injector listening on %s; upstream %s", *listen, u.Host)
	if e := srv.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		log.Fatal(fmt.Errorf("injector: %w", e))
	}
}
