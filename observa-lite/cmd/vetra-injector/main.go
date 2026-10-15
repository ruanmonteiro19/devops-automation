// Vetra injector is an opt-in edge proxy. It does not instrument native mobile apps.
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"flag"
	"fmt"
	htmlparser "golang.org/x/net/html"
	"html"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

type restoredBody struct {
	io.Reader
	io.Closer
}

func headEnd(b []byte) int {
	z := htmlparser.NewTokenizer(bytes.NewReader(b))
	offset := 0
	templates := 0
	for {
		kind := z.Next()
		offset += len(z.Raw())
		if kind == htmlparser.ErrorToken {
			return -1
		}
		if kind == htmlparser.EndTagToken {
			name, _ := z.TagName()
			if bytes.EqualFold(name, []byte("template")) && templates > 0 {
				templates--
			}
		}
		if kind == htmlparser.StartTagToken {
			name, _ := z.TagName()
			if bytes.EqualFold(name, []byte("template")) {
				templates++
				continue
			}
			if templates == 0 && bytes.EqualFold(name, []byte("head")) {
				return offset
			}
		}
	}
}

func injector(upstream *url.URL, script string, excluded []string, cspApproved bool) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Transport = &http.Transport{Proxy: nil, DisableCompression: true, MaxIdleConns: 128, MaxIdleConnsPerHost: 32, MaxConnsPerHost: 64, IdleConnTimeout: 90 * time.Second, ResponseHeaderTimeout: 15 * time.Second, TLSHandshakeTimeout: 10 * time.Second}

	baseDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		host := r.Host
		baseDirector(r)
		r.Header.Del("Forwarded")
		r.Header.Del("X-Forwarded-For")
		r.Header.Set("X-Forwarded-Host", host)
		proto := "http"
		if r.TLS != nil {
			proto = "https"
		}
		r.Header.Set("X-Forwarded-Proto", proto)
		r.Host = upstream.Host
		if script != "" {
			r.Header.Del("Accept-Encoding")
		}
		if script != "" && strings.Contains(r.Header.Get("Accept"), "text/html") {
			r.Header.Del("If-None-Match")
			r.Header.Del("If-Modified-Since")
		}
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		http.Error(w, "Application upstream unavailable", 502)
	}
	proxy.ModifyResponse = func(r *http.Response) error {
		if strings.Contains(strings.ToLower(r.Header.Get("Cache-Control")), "no-transform") || strings.Contains(strings.ToLower(r.Header.Get("Content-Disposition")), "attachment") || r.Request.Method == "HEAD" || r.Request.Header.Get("Range") != "" || r.StatusCode != 200 || !strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "text/html") {
			return nil
		}
		for _, p := range excluded {
			if p != "" && strings.HasPrefix(r.Request.URL.Path, p) {
				r.Header.Set("X-Vetra-Injection", "excluded")
				return nil
			}
		}
		coding := strings.ToLower(r.Header.Get("Content-Encoding"))
		if coding != "" && coding != "identity" && coding != "gzip" {
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
			r.Body = restoredBody{io.MultiReader(bytes.NewReader(b), r.Body), r.Body}
			r.Header.Set("X-Vetra-Injection", "skipped-size")
			return nil
		}
		_ = r.Body.Close()
		original := b
		if coding == "gzip" {
			gz, err := gzip.NewReader(bytes.NewReader(b))
			if err != nil {
				r.Body = io.NopCloser(bytes.NewReader(original))
				r.Header.Set("X-Vetra-Injection", "skipped-invalid-gzip")
				return nil
			}
			decoded, err := io.ReadAll(io.LimitReader(gz, 8<<20+1))
			gz.Close()
			if err != nil || len(decoded) > 8<<20 {
				r.Body = io.NopCloser(bytes.NewReader(original))
				r.Header.Set("X-Vetra-Injection", "skipped-size-or-gzip")
				return nil
			}
			b = decoded
		}
		if !utf8.Valid(b) {
			r.Body = io.NopCloser(bytes.NewReader(original))
			r.Header.Set("X-Vetra-Injection", "skipped-charset")
			return nil
		}
		result := b
		status := "skipped-html"
		lower := strings.ToLower(string(b))
		at := headEnd(b)
		if strings.Contains(lower, "content-security-policy") && !cspApproved {
			status = "skipped-meta-csp"
		} else if strings.Contains(lower, "data-vetra-injected") || strings.Contains(lower, "/rum/vetra.js") {
			status = "already-present"
		} else if at >= 0 {
			tag := []byte(`<script data-vetra-injected="true" src="` + html.EscapeString(script) + `" crossorigin="anonymous"></script>`)
			result = make([]byte, 0, len(b)+len(tag))
			result = append(result, b[:at]...)
			result = append(result, tag...)
			result = append(result, b[at:]...)
			status = "injected"
		}
		if status != "injected" {
			result = original
		} else if coding == "gzip" {
			var compressed bytes.Buffer
			gz := gzip.NewWriter(&compressed)
			if _, err := gz.Write(result); err != nil {
				return err
			}
			if err := gz.Close(); err != nil {
				return err
			}
			result = compressed.Bytes()
		}
		r.Body = io.NopCloser(bytes.NewReader(result))
		r.ContentLength = int64(len(result))
		r.Header.Set("Content-Length", strconv.Itoa(len(result)))
		r.Header.Set("X-Vetra-Injection", status)
		if status == "injected" {
			for _, k := range []string{"ETag", "Last-Modified", "Content-MD5", "Digest", "Content-Digest"} {
				r.Header.Del(k)
				r.Trailer.Del(k)
			}
			r.Header.Set("Cache-Control", "no-store")
		}
		return nil
	}
	if script == "" {
		proxy.ModifyResponse = nil
	}
	return proxy
}
func main() {
	configFile := flag.String("config", "", "JSON routes configuration, atomically reloaded every 5s")
	maxInflight := flag.Int("max-inflight", 8, "concurrent application requests per replica (1..4096)")
	listen := flag.String("listen", ":8080", "HTTP listener; terminate public TLS at the ingress")
	raw := flag.String("upstream", "", "fixed upstream application URL")
	script := flag.String("script-url", "", "absolute Vetra browser SDK URL with ?app=frontend-id")
	exclude := flag.String("exclude-paths", "/login,/auth,/checkout,/payment", "comma-separated URL prefixes excluded from injection")
	csp := flag.Bool("csp-approved", false, "operator confirms existing CSP permits script origin and collector; CSP is preserved")
	flag.Parse()
	if *maxInflight < 1 || *maxInflight > 4096 {
		log.Fatal("invalid max-inflight")
	}
	var handler http.Handler
	var managed *routeManager
	if *configFile != "" {
		var err error
		managed, err = newRouteManager(*configFile)
		if err != nil {
			log.Fatal(err)
		}
		handler = managed
	} else {
		u, e := url.Parse(*raw)
		s, se := url.Parse(*script)
		if e != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || se != nil || s.Hostname() == "" || s.User != nil || (s.Scheme != "http" && s.Scheme != "https") || s.Query().Get("app") == "" {
			log.Fatal("valid fixed upstream and public script-url with app ID are required")
		}
		handler = injector(u, *script, strings.Split(*exclude, ","), *csp)
	}
	handler = bounded(handler, *maxInflight)
	srv := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32768}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if managed != nil {
		go managed.watch(ctx)
	}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	log.Printf("Vetra HTML injector listening on %s", *listen)
	if e := srv.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		log.Fatal(fmt.Errorf("injector: %w", e))
	}
}
