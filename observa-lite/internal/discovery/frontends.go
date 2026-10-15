package discovery

import (
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Probe only listening sockets owned by observed processes, on loopback. No redirects or credentials.
func DiscoverFrontends(ctx context.Context, d model.Discovery) []model.FrontendCandidate {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 400 * time.Millisecond, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	out := []model.FrontendCandidate{}
	processes := map[int]model.Process{}
	for _, p := range d.Processes {
		processes[p.PID] = p
	}
	seen := map[int]bool{}
	attempts := 0
	metadataAttempts := 0
	goProcesses := map[int]bool{}
	for _, c := range d.Connections {
		if c.State != "LISTEN" || c.LocalPort < 1 || c.LocalPort > 65535 || seen[c.LocalPort] || attempts >= 20 || ctx.Err() != nil {
			continue
		}
		p, ok := processes[c.PID]
		if !ok || model.IsSystemProcess(p) {
			continue
		}
		web := p.Runtime == "java" || p.Runtime == "dotnet" || p.Runtime == "nodejs" || p.Runtime == "python" || p.Runtime == "ruby" || p.Runtime == "php" || p.Runtime == "go" || p.Runtime == "nginx" || p.Name == "httpd" || p.Name == "apache2" || p.Name == "caddy"
		if !web && p.Runtime == "process" && metadataAttempts < 10 {
			if found, checked := goProcesses[p.PID]; checked {
				web = found
			} else {
				metadataAttempts++
				_, err := buildinfo.ReadFile(fmt.Sprintf("/proc/%d/exe", p.PID))
				web = err == nil
				goProcesses[p.PID] = web
			}
		}
		if !web {
			continue
		}
		seen[c.LocalPort] = true
		attempts++
		host := "127.0.0.1"
		if strings.Contains(c.LocalIP, ":") {
			host = "::1"
		}
		scheme := "http"
		if c.LocalPort == 443 || c.LocalPort == 8443 {
			scheme = "https"
		}
		req, e := http.NewRequestWithContext(ctx, "GET", scheme+"://"+net.JoinHostPort(host, fmt.Sprint(c.LocalPort))+"/", nil)
		if e != nil {
			continue
		}
		req.Header.Set("User-Agent", "Vetra-Frontend-Discovery/1.0")
		req.Header.Set("Accept", "text/html")
		resp, e := client.Do(req)
		if e != nil {
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		html := strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") && strings.Contains(strings.ToLower(string(b)), "<html")
		if !html || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", d.HostID, c.LocalPort, p.Service)))
		out = append(out, model.FrontendCandidate{ID: fmt.Sprintf("web-%x", sum[:6]), HostID: d.HostID, HostName: d.Hostname, Service: p.Service, PID: p.PID, Port: c.LocalPort, Scheme: scheme, Evidence: "HTTP GET loopback: resposta HTML 2xx de porta vinculada ao processo", ObservedAt: time.Now().UTC()})
	}
	return out
}
