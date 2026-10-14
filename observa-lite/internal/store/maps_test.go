package store

import (
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"testing"
	"time"
)

func TestBuildMaps(t *testing.T) {
	hosts := []model.Host{{ID: "a", Name: "alpha", Addresses: []string{"10.1.1.1"}, LastSeen: time.Now(), Processes: []model.Process{{PID: 1, Service: "api"}}, Connections: []model.Connection{{PID: 1, LocalIP: "10.1.1.1", LocalPort: 34001, RemoteIP: "10.1.1.2", RemotePort: 5432, State: "ESTABLISHED"}}}, {ID: "b", Name: "bravo", Addresses: []string{"10.1.1.2"}, LastSeen: time.Now(), Processes: []model.Process{{PID: 5, Service: "postgres"}}, Connections: []model.Connection{{PID: 5, LocalIP: "0.0.0.0", LocalPort: 5432, State: "LISTEN"}}}}
	svc, net := BuildMaps(hosts)
	if len(svc.Edges) != 1 || svc.Edges[0].To != "svc:b:postgres" {
		t.Fatalf("service map %+v", svc.Edges)
	}
	if len(net.Edges) != 1 {
		t.Fatalf("network map %+v", net.Edges)
	}
}
