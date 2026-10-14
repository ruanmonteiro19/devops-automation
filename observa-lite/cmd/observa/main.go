package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"regexp"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/discovery"
	"github.com/ruanmonteiro19/observa-lite/internal/server"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: observa server|agent|once")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "server":
		err = runServer(os.Args[2:])
	case "health":
		err = runHealth(os.Args[2:])
	case "agent":
		err = runAgent(os.Args[2:])
	case "once":
		d, e := discovery.Detect()
		if e != nil {
			err = e
		} else {
			err = json.NewEncoder(os.Stdout).Encode(d)
		}
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}
func runServer(args []string) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	addr := fs.String("listen", env("OBSERVA_LISTEN", "127.0.0.1:8080"), "HTTP listen address (use a reverse proxy for TLS)")
	data := fs.String("data", env("OBSERVA_DB", "./observa-data/state.json"), "state JSON path")
	if e := fs.Parse(args); e != nil {
		return e
	}
	st, e := store.New(*data)
	if e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	return server.New(st, os.Getenv("OBSERVA_ADMIN_TOKEN"), os.Getenv("OBSERVA_AGENT_TOKEN"), server.AllowPrivateFromEnv()).Listen(ctx, *addr)
}
// runHealth is a shell-free health check for distroless Docker images.
func runHealth(args []string) error {
    fs := flag.NewFlagSet("health", flag.ContinueOnError)
    endpoint := fs.String("url", env("OBSERVA_HEALTH_URL", "http://127.0.0.1:8080/healthz"), "healthcheck URL")
    if err := fs.Parse(args); err != nil { return err }
    client := &http.Client{Timeout: 3 * time.Second}
    resp, err := client.Get(*endpoint)
    if err != nil { return err }
    defer resp.Body.Close()
    _, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("healthcheck failed: HTTP %d", resp.StatusCode)
    }
    return nil
}

func runAgent(args []string) error {
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	endpoint := fs.String("server", env("OBSERVA_SERVER", "http://127.0.0.1:8080"), "Observa server root URL")
	interval := fs.Duration("interval", 30*time.Second, "discovery interval; minimum 10s")
 hostGroup := fs.String("host-group",env("OBSERVA_HOST_GROUP",""),"logical host group")
 networkZone := fs.String("network-zone",env("OBSERVA_NETWORK_ZONE","default"),"network routing zone")
 mode := fs.String("mode",env("OBSERVA_MONITORING_MODE","discovery"),"supported: discovery, off; infrastructure/deep planned")
 caFile := fs.String("ca-file",env("OBSERVA_CA_FILE",""),"PEM CA bundle for trusted gateway")
 clientCert := fs.String("client-cert",env("OBSERVA_CLIENT_CERT",""),"optional PEM client certificate for mTLS")
 clientKey := fs.String("client-key",env("OBSERVA_CLIENT_KEY",""),"optional PEM client private key for mTLS")
 legacyPayload := fs.Bool("legacy-payload",env("OBSERVA_LEGACY_PAYLOAD","false")=="true","send old schema without group/zone, for older servers only")
	if e := fs.Parse(args); e != nil {
		return e
	}
	if *mode=="off" {return fmt.Errorf("monitoring is disabled; do not start agent until enabled")}
 if *mode!="discovery" {return fmt.Errorf("mode %q is not implemented yet; only discovery mode is functional",*mode)}
 namePattern:=regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9_.-]*$")
 if (*hostGroup!=""&&(len(*hostGroup)>100||!namePattern.MatchString(*hostGroup)))||(*networkZone!=""&&(len(*networkZone)>256||!namePattern.MatchString(*networkZone))){return fmt.Errorf("invalid host-group or network-zone")}
	if *interval < 10*time.Second {
		return fmt.Errorf("minimum interval 10s")
	}
	token := os.Getenv("OBSERVA_AGENT_TOKEN")
	if len(token) < 24 {
		return fmt.Errorf("OBSERVA_AGENT_TOKEN must be at least 24 characters")
	}
	if !(strings.HasPrefix(*endpoint, "https://") || strings.HasPrefix(*endpoint, "http://127.0.0.1:") || strings.HasPrefix(*endpoint, "http://localhost:")) {
		return fmt.Errorf("agent requires HTTPS for remote ingestion")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	tr:=http.DefaultTransport.(*http.Transport).Clone()
 tr.TLSClientConfig=&tls.Config{MinVersion:tls.VersionTLS12}
 if *caFile!=""{
  pem,err:=os.ReadFile(*caFile);if err!=nil{return err}
  roots,err:=x509.SystemCertPool();if err!=nil||roots==nil{roots=x509.NewCertPool()}
  if !roots.AppendCertsFromPEM(pem){return fmt.Errorf("invalid PEM CA file")}
  tr.TLSClientConfig.RootCAs=roots
 }
 if (*clientCert=="")!=(*clientKey==""){return fmt.Errorf("client-cert and client-key must both be set")}
 if *clientCert!=""{
  cert,err:=tls.LoadX509KeyPair(*clientCert,*clientKey);if err!=nil{return err}
  tr.TLSClientConfig.Certificates=[]tls.Certificate{cert}
 }
 client := &http.Client{Timeout: 15 * time.Second,Transport:tr}
	for {
		d, e := discovery.Detect()
		if e == nil {
           d.HostGroup=*hostGroup
           d.NetworkZone=*networkZone
           d.MonitoringMode=*mode
           if *legacyPayload {d.HostGroup="";d.NetworkZone="";d.MonitoringMode=""}
			var b []byte
			b, e = json.Marshal(d)
			if e == nil {
				req, _ := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(*endpoint, "/")+"/api/v1/ingest", bytes.NewReader(b))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-API-Key", token)
				var resp *http.Response
				resp, e = client.Do(req)
				if e == nil {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
					resp.Body.Close()
					if resp.StatusCode != 202 {
						e = fmt.Errorf("ingest: %d %s", resp.StatusCode, string(body))
					}
				}
			}
		}
		if e != nil {
			log.Printf("agent: %v", e)
		} else {
			log.Printf("sent %d processes and %d sockets", len(d.Processes), len(d.Connections))
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(*interval):
		}
	}
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
