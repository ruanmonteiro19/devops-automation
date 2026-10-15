package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var rumPublicPath = regexp.MustCompile(`^/rum/(config|beacon)/[A-Za-z0-9_-]{1,96}$`)

func publicRUM(method, path string) bool {
	if method == http.MethodGet && path == "/rum/vetra.js" {
		return true
	}
	if !rumPublicPath.MatchString(path) {
		return false
	}
	if strings.HasPrefix(path, "/rum/config/") {
		return method == http.MethodGet
	}
	return method == http.MethodPost || method == http.MethodOptions
}

type relay struct {
	upstream                 *url.URL
	httpClient               *http.Client
	token                    string
	slots                    chan struct{}
	requireClientCertificate bool
}

func (r *relay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/healthz" && req.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"ok\":true}"))
		return
	}
	isRUM := publicRUM(req.Method, req.URL.Path)
	isOTLP := req.URL.Path == "/v1/traces" || req.URL.Path == "/v1/logs" || req.URL.Path == "/v1/metrics"
	if !isRUM && (req.Method != http.MethodPost || (req.URL.Path != "/api/v1/ingest" && !isOTLP)) {
		http.NotFound(w, req)
		return
	}
	if !isRUM && r.requireClientCertificate && (req.TLS == nil || len(req.TLS.VerifiedChains) == 0) {
		http.Error(w, "verified agent client certificate required", 403)
		return
	}
	key := req.Header.Get("X-API-Key")
	if isOTLP && strings.HasPrefix(strings.ToLower(req.Header.Get("Authorization")), "bearer ") {
		key = req.Header.Get("Authorization")[7:]
	}
	if !isRUM && (len(key) != len(r.token) || subtle.ConstantTimeCompare([]byte(key), []byte(r.token)) != 1) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	select {
	case r.slots <- struct{}{}:
		defer func() { <-r.slots }()
	default:
		w.Header().Set("Retry-After", "1")
		http.Error(w, "gateway busy", http.StatusServiceUnavailable)
		return
	}
	req.Body = http.MaxBytesReader(w, req.Body, 6<<20)
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "invalid or oversized body", http.StatusRequestEntityTooLarge)
		return
	}
	upstream := r.upstream.JoinPath("api", "v1", "ingest")
	if isRUM {
		upstream = r.upstream.JoinPath(req.URL.Path)
		upstream.RawQuery = req.URL.RawQuery
	}
	if isOTLP {
		upstream = r.upstream.JoinPath("v1", strings.TrimPrefix(req.URL.Path, "/v1/"))
		upstream.RawQuery = req.URL.RawQuery
	}
	forwarded, err := http.NewRequestWithContext(req.Context(), req.Method, upstream.String(), bytes.NewReader(body))
	if err != nil {
		http.Error(w, "invalid forwarding request", http.StatusBadGateway)
		return
	}
	forwarded.Header.Set("Content-Type", "application/json")
	if isOTLP {
		forwarded.Header.Set("Content-Type", req.Header.Get("Content-Type"))
		if encoding := req.Header.Get("Content-Encoding"); encoding != "" {
			forwarded.Header.Set("Content-Encoding", encoding)
		}
	}
	if isRUM {
		for _, key := range []string{"Content-Type", "Origin", "Referer", "Accept", "User-Agent"} {
			if v := req.Header.Get(key); v != "" {
				forwarded.Header.Set(key, v)
			}
		}
		// Browser credentials and administrative tokens are never forwarded to the public collector.
	} else {
		forwarded.Header.Set("X-API-Key", r.token)
	}
	resp, err := r.httpClient.Do(forwarded)
	if err != nil {
		log.Printf("gateway upstream failure: %v", err)
		http.Error(w, "backend unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if isRUM {
		for _, key := range []string{"Content-Type", "Access-Control-Allow-Origin", "Access-Control-Allow-Methods", "Access-Control-Allow-Headers", "Access-Control-Max-Age", "Vary", "Cache-Control", "X-Content-Type-Options", "Retry-After"} {
			for _, v := range resp.Header.Values(key) {
				w.Header().Add(key, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, io.LimitReader(resp.Body, 8<<20))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if isOTLP {
		w.Header().Del("Content-Type")
		for _, key := range []string{"Content-Type", "Content-Encoding", "Retry-After"} {
			if value := resp.Header.Get(key); value != "" {
				w.Header().Set(key, value)
			}
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 4096))
}

func loadRoots(p string) (*x509.CertPool, error) {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if p == "" {
		return roots, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	if !roots.AppendCertsFromPEM(b) {
		return nil, errors.New("invalid PEM CA")
	}
	return roots, nil
}
func localURL(u *url.URL) bool {
	host := u.Hostname()
	ip := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
}
func main() {
	publicRUMWithMTLS := flag.Bool("public-rum-with-mtls", false, "allow public browser RUM without client certificate while enforcing verified mTLS on agent/OTLP routes")
	maxInflight := flag.Int("max-inflight", 32, "maximum concurrent forwarding requests per replica (1..4096)")
	listen := flag.String("listen", ":18443", "TCP listen address")
	upstreamRaw := flag.String("upstream", "http://127.0.0.1:8080", "backend server URL (HTTPS required remotely)")
	cert := flag.String("tls-cert", "", "gateway TLS certificate with correct SAN")
	key := flag.String("tls-key", "", "gateway TLS private key")
	clientCA := flag.String("client-ca", "", "optional client certificate CA; requires client mTLS")
	backendCA := flag.String("upstream-ca", "", "optional backend CA certificate")
	flag.Parse()
	if *maxInflight < 1 || *maxInflight > 4096 {
		log.Fatal("max-inflight must be between 1 and 4096")
	}
	token := os.Getenv("OBSERVA_AGENT_TOKEN")
	if len(token) < 24 || *cert == "" || *key == "" {
		log.Fatal("agent token >=24 bytes and TLS certificate/key required")
	}
	u, err := url.Parse(*upstreamRaw)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && !(u.Scheme == "http" && localURL(u))) {
		log.Fatal("upstream must be HTTPS, or loopback HTTP on the same machine")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		log.Fatal("upstream URL must not include query/fragment")
	}
	roots, err := loadRoots(*backendCA)
	if err != nil {
		log.Fatal(err)
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	client := &http.Client{Transport: tr, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect denied") }}
	auth := &tls.Config{MinVersion: tls.VersionTLS12}
	if *clientCA != "" {
		ca, err := os.ReadFile(*clientCA)
		if err != nil {
			log.Fatal(err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(ca) {
			log.Fatal("invalid client CA")
		}
		auth.ClientCAs = roots
		auth.ClientAuth = tls.RequireAndVerifyClientCert
		if *publicRUMWithMTLS {
			auth.ClientAuth = tls.VerifyClientCertIfGiven
		}
	}
	h := &relay{requireClientCertificate: *clientCA != "" && *publicRUMWithMTLS, upstream: u, httpClient: client, token: token, slots: make(chan struct{}, *maxInflight)}
	srv := &http.Server{Addr: *listen, Handler: h, TLSConfig: auth, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 20 * time.Second, MaxHeaderBytes: 16384}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	log.Printf("Observa Gateway listening on %s; upstream %s; mTLS %t", *listen, u.Host, *clientCA != "")
	if err := srv.ListenAndServeTLS(*cert, *key); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(fmt.Errorf("gateway: %w", err))
	}
}
