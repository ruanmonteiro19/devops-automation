package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRelayAuthenticatedAndRestricted(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/ingest" || r.Header.Get("X-API-Key") != "012345678901234567890123456789" {
			t.Errorf("bad forwarded data")
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	r := &relay{upstream: u, httpClient: upstream.Client(), token: "012345678901234567890123456789", slots: make(chan struct{}, 2)}
	for _, tc := range []struct {
		method, path, token string
		status              int
	}{
		{"GET", "/healthz", "", 200},
		{"POST", "/api/v1/ingest", "bad", 401},
		{"POST", "/api/v1/anything", "012345678901234567890123456789", 404},
		{"POST", "/api/v1/ingest", "012345678901234567890123456789", 202},
	} {
		q := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString("{}")).WithContext(context.Background())
		q.Header.Set("X-API-Key", tc.token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, q)
		if w.Code != tc.status {
			t.Errorf("%s %s: got %d want %d", tc.method, tc.path, w.Code, tc.status)
		}
	}
}

func TestRelayOTLPWireFormats(t *testing.T) {
	const token = "agent-token-12345678901234567890"
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write([]byte{0x0a, 0x00}); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, contentType, encoding, query string
		body, response                     []byte
		status                             int
	}{
		{"json", "application/json", "", "", []byte(`{"resourceSpans":[]}`), []byte("{}"), 200},
		{"protobuf", "application/x-protobuf", "", "", []byte{0x0a, 0x00}, nil, 200},
		{"gzip protobuf with query", "application/x-protobuf", "gzip", "?tenant=blue&x=1%202", compressed.Bytes(), []byte{0x08, 0x01}, 429},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls++
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Error(err)
				}
				if req.Method != "POST" || req.URL.RequestURI() != "/v1/traces"+tc.query || req.Header.Get("X-API-Key") != token || req.Header.Get("Content-Type") != tc.contentType || req.Header.Get("Content-Encoding") != tc.encoding || !bytes.Equal(body, tc.body) {
					t.Errorf("relay changed OTLP request: %s %s %#v %x", req.Method, req.URL.RequestURI(), req.Header, body)
				}
				w.Header().Set("Content-Type", tc.contentType)
				w.Header().Set("Retry-After", "7")
				w.WriteHeader(tc.status)
				_, _ = w.Write(tc.response)
			}))
			defer upstream.Close()
			u, _ := url.Parse(upstream.URL)
			relay := &relay{upstream: u, httpClient: upstream.Client(), token: token, slots: make(chan struct{}, 1)}
			req := httptest.NewRequest(http.MethodPost, "/v1/traces"+tc.query, bytes.NewReader(tc.body))
			req.Header.Set("X-API-Key", token)
			req.Header.Set("Content-Type", tc.contentType)
			req.Header.Set("Content-Encoding", tc.encoding)
			rr := httptest.NewRecorder()
			relay.ServeHTTP(rr, req)
			if calls != 1 || rr.Code != tc.status || rr.Header().Get("Content-Type") != tc.contentType || rr.Header().Get("Retry-After") != "7" || !bytes.Equal(rr.Body.Bytes(), tc.response) {
				t.Fatalf("relay changed OTLP response: calls=%d status=%d headers=%#v body=%x", calls, rr.Code, rr.Header(), rr.Body.Bytes())
			}
		})
	}
}
func TestRelayOTLPAuthorizationAndPathRestrictions(t *testing.T) {
	const token = "agent-token-12345678901234567890"
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(200)
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	relay := &relay{upstream: u, httpClient: upstream.Client(), token: token, slots: make(chan struct{}, 1)}
	for _, tc := range []struct {
		method, path, key, bearer string
		want                      int
	}{
		{"POST", "/v1/traces", "", "", 401},
		{"POST", "/v1/traces", "admin-token-12345678901234567890", "", 401},
		{"POST", "/v1/traces", "bad", "", 401},
		{"POST", "/v1/traces", token, "", 200},
		{"POST", "/v1/logs", token, "", 200},
		{"POST", "/v1/metrics", token, "", 200},
		{"POST", "/v1/logs", "", "", 401},
		{"POST", "/v1/metrics", "", "", 401},
		{"POST", "/v1/traces", "", token, 200},
		{"POST", "/v1/traces", token, "wrong", 401},
		{"GET", "/v1/traces", token, "", 404},
		{"POST", "/api/v1/apm/services", token, "", 404},
		{"POST", "/api/v1/dashboards", token, "", 404},
		{"POST", "/v1/traces/admin", token, "", 404},
		{"POST", "/v1/traces/", token, "", 404},
	} {
		before := calls
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(nil))
		req.Header.Set("X-API-Key", tc.key)
		if tc.bearer != "" {
			req.Header.Set("Authorization", "Bearer "+tc.bearer)
		}
		rr := httptest.NewRecorder()
		relay.ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Errorf("%s %s returned %d want %d", tc.method, tc.path, rr.Code, tc.want)
		}
		if tc.want != 200 && calls != before {
			t.Error("rejected request reached backend")
		}
	}
}

func TestRelayLogsMetricsPreservePathAndWire(t *testing.T) {
	const token = "agent-token-12345678901234567890"
	for _, path := range []string{"/v1/logs", "/v1/metrics"} {
		t.Run(path, func(t *testing.T) {
			body := []byte{0x0a, 0x01, 0xff}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				got, err := io.ReadAll(req.Body)
				if err != nil {
					t.Error(err)
				}
				if req.URL.RequestURI() != "/prefix"+path+"?tenant=blue" || req.Header.Get("Content-Type") != "application/x-protobuf" || req.Header.Get("Content-Encoding") != "gzip" || !bytes.Equal(body, got) {
					t.Errorf("signal wire changed: %s %#v %x", req.URL.RequestURI(), req.Header, got)
				}
				w.Header().Set("Content-Type", "application/x-protobuf")
				w.WriteHeader(200)
			}))
			defer upstream.Close()
			u, _ := url.Parse(upstream.URL + "/prefix")
			relay := &relay{upstream: u, httpClient: upstream.Client(), token: token, slots: make(chan struct{}, 1)}
			req := httptest.NewRequest("POST", path+"?tenant=blue", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/x-protobuf")
			req.Header.Set("Content-Encoding", "gzip")
			rr := httptest.NewRecorder()
			relay.ServeHTTP(rr, req)
			if rr.Code != 200 || rr.Header().Get("Content-Type") != "application/x-protobuf" || rr.Body.Len() != 0 {
				t.Fatalf("signal relay failed: %d %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestRelayBackpressureSignalsRetry(t *testing.T) {
	r := &relay{token: "test-token", slots: make(chan struct{}, 1)}
	r.slots <- struct{}{}
	req := httptest.NewRequest("POST", "/api/v1/ingest", bytes.NewBufferString("{}"))
	req.Header.Set("X-API-Key", "test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 503 || w.Header().Get("Retry-After") != "1" {
		t.Fatalf("missing overload signal: %d %v", w.Code, w.Header())
	}
}

func TestRelayPublicRUMDoesNotExposeAdminOrTruncateSDK(t *testing.T) {
	const token = "agent-token-12345678901234567890"
	sdk := strings.Repeat("x", 200000)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("browser credentials leaked")
		}
		if r.Header.Get("Origin") != "https://portal.example" {
			t.Error("lost CORS origin")
		}
		w.Header().Set("Access-Control-Allow-Origin", "https://portal.example")
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Content-Type", "application/javascript")
		if r.URL.Path == "/rum/vetra.js" {
			io.WriteString(w, sdk)
		} else {
			w.WriteHeader(202)
		}
	}))
	defer backend.Close()
	u, _ := url.Parse(backend.URL)
	gateway := &relay{upstream: u, httpClient: backend.Client(), token: token, slots: make(chan struct{}, 2)}
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"GET", "/rum/vetra.js?app=portal", 200}, {"GET", "/rum/config/portal", 202}, {"POST", "/rum/beacon/portal", 202}, {"OPTIONS", "/rum/beacon/portal", 202}, {"GET", "/api/v1/snapshot", 404}, {"POST", "/mcp", 404}, {"GET", "/rum/config/../admin", 404}, {"POST", "/rum/config/portal", 404}, {"GET", "/rum/beacon/portal", 404}} {
		q := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
		q.Header.Set("Origin", "https://portal.example")
		q.Header.Set("Cookie", "session=private")
		q.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		gateway.ServeHTTP(w, q)
		if w.Code != tc.status {
			t.Fatalf("%s %s: %d", tc.method, tc.path, w.Code)
		}
		if tc.path == "/rum/vetra.js?app=portal" && (w.Body.String() != sdk || w.Header().Get("Access-Control-Allow-Origin") != "https://portal.example") {
			t.Fatal("SDK/CORS changed")
		}
	}
}

func TestPublicRUMWithMTLSStillRequiresAgentCertificate(t *testing.T) {
	const token = "agent-token-12345678901234567890"
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(202) }))
	defer backend.Close()
	u, _ := url.Parse(backend.URL)
	gateway := &relay{upstream: u, httpClient: backend.Client(), token: token, slots: make(chan struct{}, 2), requireClientCertificate: true}
	for _, tc := range []struct {
		path string
		cert bool
		want int
	}{{"/api/v1/ingest", false, 403}, {"/v1/traces", false, 403}, {"/api/v1/ingest", true, 202}, {"/rum/beacon/portal", false, 202}} {
		q := httptest.NewRequest("POST", tc.path, strings.NewReader("{}"))
		q.Header.Set("X-API-Key", token)
		if tc.cert {
			q.TLS = &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{{}}}}
		}
		w := httptest.NewRecorder()
		gateway.ServeHTTP(w, q)
		if w.Code != tc.want {
			t.Errorf("%s cert=%t: %d", tc.path, tc.cert, w.Code)
		}
	}
}
