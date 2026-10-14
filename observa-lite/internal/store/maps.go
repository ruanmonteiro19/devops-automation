package store

import (
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net"
	"sort"
	"strings"
	"time"
)

func BuildMaps(hosts []model.Host) (model.Map, model.Map) {
	services := model.Map{Nodes: []model.Node{}, Edges: []model.Edge{}}
	networks := model.Map{Nodes: []model.Node{}, Edges: []model.Edge{}}
	sn := map[string]*model.Node{}
	nn := map[string]*model.Node{}
	se := map[string]*model.Edge{}
	ne := map[string]*model.Edge{}
	pidService := map[string]string{}
	ipHost := map[string]string{}
	listeners := map[string]string{}
	for _, h := range hosts {
		hn := "host:" + h.ID
		nn[hn] = &model.Node{ID: hn, Name: h.Name, Kind: "host", Status: hostStatus(h)}
		for _, ip := range h.Addresses {
			ipHost[ip] = hn
		}
		for _, p := range h.Processes {
			svc := strings.TrimSpace(p.Service)
			if svc == "" {
				continue
			}
			id := "svc:" + h.ID + ":" + svc
			pidService[fmt.Sprintf("%s/%d", h.ID, p.PID)] = id
			if _, ok := sn[id]; !ok {
				sn[id] = &model.Node{ID: id, Name: svc, Kind: "service", Host: h.Name, Runtime: p.Runtime, Status: hostStatus(h)}
			}
			sn[id].RSSBytes += p.RSSBytes
			sn[id].Instances++
		}
		for _, c := range h.Connections {
			if c.State == "LISTEN" {
				id := pidService[fmt.Sprintf("%s/%d", h.ID, c.PID)]
				if id == "" {
					continue
				}
				listeners[h.ID+":"+fmt.Sprint(c.LocalPort)] = id
				for _, ip := range h.Addresses {
					listeners[ip+":"+fmt.Sprint(c.LocalPort)] = id
				}
				listeners[c.LocalIP+":"+fmt.Sprint(c.LocalPort)] = id
			}
		}
	}
	for _, h := range hosts {
		fromHost := "host:" + h.ID
		for _, c := range h.Connections {
			if c.State == "LISTEN" || c.RemotePort == 0 || (c.RemoteIP == "0.0.0.0" || c.RemoteIP == "::") {
				continue
			}
			if c.State != "ESTABLISHED" && c.State != "SYN_SENT" && c.State != "CLOSE_WAIT" {
				continue
			}
			source := pidService[fmt.Sprintf("%s/%d", h.ID, c.PID)]
			if source == "" {
				continue
			}
			target := listeners[c.RemoteIP+":"+fmt.Sprint(c.RemotePort)]
			targetHost := ipHost[c.RemoteIP]
			if targetHost == "" && net.ParseIP(c.RemoteIP) != nil && net.ParseIP(c.RemoteIP).IsLoopback() {
				targetHost = fromHost
				target = listeners[h.ID+":"+fmt.Sprint(c.RemotePort)]
			}
			if target == "" {
				target = "ext:" + c.RemoteIP + ":" + fmt.Sprint(c.RemotePort)
				if _, ok := sn[target]; !ok {
					sn[target] = &model.Node{ID: target, Name: c.RemoteIP + ":" + fmt.Sprint(c.RemotePort), Kind: "external", Status: "unknown"}
				}
			}
			if source != target {
				k := source + "->" + target
				if _, ok := se[k]; !ok {
					se[k] = &model.Edge{ID: k, From: source, To: target, Kind: "tcp", Evidence: "socket TCP observado", Confidence: "observed"}
				}
				se[k].Count++
			}
			if targetHost == "" {
				targetHost = "ip:" + c.RemoteIP
				if _, ok := nn[targetHost]; !ok {
					nn[targetHost] = &model.Node{ID: targetHost, Name: c.RemoteIP, Kind: "external-host", Status: "unknown"}
				}
			}
			if fromHost != targetHost {
				k := fromHost + "->" + targetHost
				if _, ok := ne[k]; !ok {
					ne[k] = &model.Edge{ID: k, From: fromHost, To: targetHost, Kind: "observed-ip-flow", Evidence: "conexão IP; não é topologia física", Confidence: "observed"}
				}
				ne[k].Count++
			}
		}
	}
	for _, n := range sn {
		services.Nodes = append(services.Nodes, *n)
	}
	for _, e := range se {
		services.Edges = append(services.Edges, *e)
	}
	for _, n := range nn {
		networks.Nodes = append(networks.Nodes, *n)
	}
	for _, e := range ne {
		networks.Edges = append(networks.Edges, *e)
	}
	sort.Slice(services.Nodes, func(i, j int) bool { return services.Nodes[i].ID < services.Nodes[j].ID })
	sort.Slice(services.Edges, func(i, j int) bool { return services.Edges[i].ID < services.Edges[j].ID })
	sort.Slice(networks.Nodes, func(i, j int) bool { return networks.Nodes[i].ID < networks.Nodes[j].ID })
	sort.Slice(networks.Edges, func(i, j int) bool { return networks.Edges[i].ID < networks.Edges[j].ID })
	return services, networks
}
func hostStatus(h model.Host) string {
	if time.Since(h.LastSeen) > 2*time.Minute {
		return "stale"
	}
	return "ok"
}
