package discovery

import (
	"context"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFrontendEvidenceAndOwnership(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<!doctype html><html><body>Test</body></html>")
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	d := model.Discovery{HostID: "test", Hostname: "Host", Processes: []model.Process{{PID: 12, Name: "node", Service: "Portal", Runtime: "nodejs"}}, Connections: []model.Connection{{PID: 12, LocalPort: port, LocalIP: "127.0.0.1", State: "LISTEN"}}}
	got := DiscoverFrontends(context.Background(), d)
	if len(got) != 1 || got[0].Service != "Portal" || got[0].Port != port {
		t.Fatalf("%+v", got)
	}
	d.Connections[0].PID = 99
	if got := DiscoverFrontends(context.Background(), d); len(got) != 0 {
		t.Fatal("accepted unowned socket")
	}
	d.Connections[0].PID = 12
	d.Processes[0].Runtime = "native"
	d.Processes[0].Name = "bash"
	d.Processes[0].Service = "bash"
	if got := DiscoverFrontends(context.Background(), d); len(got) != 0 {
		t.Fatal("accepted OS process")
	}
}
