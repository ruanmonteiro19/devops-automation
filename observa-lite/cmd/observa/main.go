package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/agent"
	"github.com/ruanmonteiro19/observa-lite/internal/discovery"
	"github.com/ruanmonteiro19/observa-lite/internal/instrumentation"
	"github.com/ruanmonteiro19/observa-lite/internal/server"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: observa server|agent|once|instrumentation|health")
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
	case "instrumentation":
		fs := flag.NewFlagSet("instrumentation", flag.ContinueOnError)
		dir := fs.String("directory", "", "export Node preload module to this directory without overwriting")
		if err = fs.Parse(os.Args[2:]); err == nil {
			var path string
			path, err = instrumentation.Export(*dir)
			if err == nil {
				fmt.Println(path)
			}
		}
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
	engine := env("VETRA_STORAGE_ENGINE", "json")
	var st *store.Store
	var e error
	if engine == "json" {
		st, e = store.New(*data)
	} else {
		st, e = store.NewSQL(*data, engine, os.Getenv("VETRA_STORAGE_DSN"))
	}
	if e != nil {
		return e
	}
	defer st.Close()
	if e = st.BootstrapUsers(); e != nil {
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
	if err := fs.Parse(args); err != nil {
		return err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(*endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

func runAgent(args []string) error { return agent.Run(args) }

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func newAgentHTTPClient(tr *http.Transport) *http.Client { return agent.NewHTTPClient(tr) }
