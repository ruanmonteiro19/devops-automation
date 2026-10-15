package collectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Runner struct {
	AllowPrivate      bool
	integrationClient func() *http.Client
}

func (r Runner) dial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, e := net.SplitHostPort(address)
	if e != nil {
		return nil, e
	}
	ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
	if e != nil {
		return nil, e
	}
	if len(ips) == 0 {
		return nil, errors.New("no IP address found")
	}
	d := net.Dialer{Timeout: 4 * time.Second}
	for _, ip := range ips {
		if !r.AllowPrivate && restricted(ip.IP) {
			continue
		}
		conn, err := d.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return conn, nil
		}
	}
	return nil, errors.New("destination denied or unreachable; enable OBSERVA_ALLOW_PRIVATE_TARGETS for trusted internal targets")
}
func restricted(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}
func (r Runner) client(timeout time.Duration) *http.Client {
	tr := &http.Transport{DialContext: r.dial, Proxy: nil, MaxIdleConns: 8, IdleConnTimeout: 10 * time.Second}
	return &http.Client{Timeout: timeout, Transport: tr, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return errors.New("too many redirects")
		}
		return nil
	}}
}
func (r Runner) httpGet(ctx context.Context, rawURL, secretEnv string, timeout time.Duration) ([]byte, int, time.Duration, error) {
	u, e := url.Parse(rawURL)
	if e != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, 0, 0, errors.New("invalid HTTP(S) target")
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if e != nil {
		return nil, 0, 0, e
	}
	if secretEnv != "" {
		token := os.Getenv(secretEnv)
		if token == "" {
			return nil, 0, 0, fmt.Errorf("credential environment variable %s is empty", secretEnv)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	start := time.Now()
	resp, e := r.client(timeout).Do(req)
	dur := time.Since(start)
	if e != nil {
		return nil, 0, dur, e
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return b, resp.StatusCode, dur, e
}
func (r Runner) Check(ctx context.Context, t model.Template) (bool, float64, string) {
	timeout := time.Duration(t.TimeoutSeconds) * time.Second
	if timeout < time.Second {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	switch t.Kind {
	case "tcp":
		target := strings.TrimPrefix(t.Target, "tcp://")
		if _, _, e := net.SplitHostPort(target); e != nil {
			return false, 0, "invalid host:port"
		}
		start := time.Now()
		conn, e := r.dial(ctx, "tcp", target)
		if e != nil {
			return false, 0, e.Error()
		}
		conn.Close()
		return true, float64(time.Since(start).Milliseconds()), ""
	case "http":
		_, code, duration, e := r.httpGet(ctx, t.Target, "", timeout)
		if e != nil {
			return false, 0, e.Error()
		}
		expected := t.ExpectedStatus
		if expected == 0 {
			expected = 200
		}
		if code != expected {
			return false, float64(duration.Milliseconds()), fmt.Sprintf("HTTP %d, esperado %d", code, expected)
		}
		return true, float64(duration.Milliseconds()), ""
	default:
		return false, 0, "template kind not supported (available: tcp, http)"
	}
}

type QueryResult struct {
	Value float64     `json:"value"`
	Text  string      `json:"text"`
	At    time.Time   `json:"at"`
	Raw   interface{} `json:"raw,omitempty"`
}

func (r Runner) Query(ctx context.Context, ds model.DataSource, expr string) (QueryResult, error) {
	timeout := 8 * time.Second
	rawURL := ds.URL
	switch ds.Kind {
	case "prometheus":
		u, e := url.Parse(rawURL)
		if e != nil || u.Hostname() == "" {
			return QueryResult{}, errors.New("bad Prometheus URL")
		}
		u.Path = strings.TrimSuffix(u.Path, "/") + "/api/v1/query"
		q := u.Query()
		q.Set("query", expr)
		u.RawQuery = q.Encode()
		rawURL = u.String()
	case "json":
	default:
		return QueryResult{}, fmt.Errorf("unsupported datasource kind: %s", ds.Kind)
	}
	body, status, _, e := r.httpGet(ctx, rawURL, ds.SecretEnv, timeout)
	if e != nil {
		return QueryResult{}, e
	}
	if status < 200 || status >= 300 {
		return QueryResult{}, fmt.Errorf("HTTP status %d", status)
	}
	var raw interface{}
	if e := json.Unmarshal(body, &raw); e != nil {
		return QueryResult{}, e
	}
	result := QueryResult{At: time.Now().UTC(), Raw: raw}
	if ds.Kind == "prometheus" {
		root, ok := raw.(map[string]interface{})
		if !ok {
			return result, errors.New("unexpected prometheus response")
		}
		data, _ := root["data"].(map[string]interface{})
		vectors, _ := data["result"].([]interface{})
		if len(vectors) == 0 {
			return result, errors.New("prometheus returned no series")
		}
		item, _ := vectors[0].(map[string]interface{})
		value, _ := item["value"].([]interface{})
		if len(value) != 2 {
			return result, errors.New("prometheus returned no instant value")
		}
		result.Text = fmt.Sprint(value[1])
		result.Value, _ = strconv.ParseFloat(result.Text, 64)
		return result, nil
	}
	// JSON datasource expr is a dot-separated object path: e.g. status.users_online
	cursor := raw
	if expr != "" {
		for _, p := range strings.Split(strings.TrimPrefix(expr, "$."), ".") {
			if p == "" {
				continue
			}
			obj, ok := cursor.(map[string]interface{})
			if !ok {
				return result, fmt.Errorf("JSON path %q is not an object", p)
			}
			v, found := obj[p]
			if !found {
				return result, fmt.Errorf("JSON field %q not found", p)
			}
			cursor = v
		}
	}
	result.Text = fmt.Sprint(cursor)
	switch v := cursor.(type) {
	case float64:
		result.Value = v
	case bool:
		if v {
			result.Value = 1
		}
	case string:
		result.Value, _ = strconv.ParseFloat(v, 64)
	}
	return result, nil
}
