package server

import (
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

func (s *Server) topologyViews(w http.ResponseWriter, r *http.Request) {
	view := r.URL.Query().Get("view")
	allowed := map[string]bool{"all": true, "infrastructure": true, "services": true, "problems": true, "kubernetes": true, "aws": true, "azure": true, "gcp": true, "ai": true, "devices": true}
	if !allowed[view] {
		http.Error(w, "unsupported topology view", 400)
		return
	}
	from, to, windowErr := apmWindow(r)
	if windowErr != nil {
		fail(w, 400, windowErr)
		return
	}
	graph := model.Map{Nodes: []model.Node{}, Edges: []model.Edge{}}
	nodes := map[string]bool{}
	edges := map[string]bool{}
	truncated := false
	node := func(v model.Node) {
		if nodes[v.ID] {
			return
		}
		if len(graph.Nodes) >= 500 {
			truncated = true
			return
		}
		nodes[v.ID] = true
		graph.Nodes = append(graph.Nodes, v)
	}
	edge := func(v model.Edge) {
		if edges[v.ID] || !nodes[v.From] || !nodes[v.To] {
			return
		}
		if len(graph.Edges) >= 2000 {
			truncated = true
			return
		}
		edges[v.ID] = true
		graph.Edges = append(graph.Edges, v)
	}
	snapshot := s.Store.Snapshot()
	if view == "all" || view == "services" || view == "problems" {
		serviceMap := s.Store.APMMap(from, to)
		for _, n := range serviceMap.Nodes {
			node(n)
		}
		for _, e := range serviceMap.Edges {
			edge(e)
		}
	}
	if view == "all" || view == "infrastructure" {
		for _, h := range snapshot.Hosts {
			node(model.Node{ID: "host:" + h.ID, Name: h.Name, Kind: "host", Status: "observed"})
		}
		for _, n := range snapshot.ProcessMap.Nodes {
			original := n.ID
			n.ID = "process/" + n.ID
			n.Kind = "process"
			node(n)
			for _, h := range snapshot.Hosts {
				if strings.HasPrefix(original, "svc:"+h.ID+":") {
					edge(model.Edge{ID: "runs/" + n.ID, From: "host:" + h.ID, To: n.ID, Kind: "contains", Count: 1, Evidence: "agent inventory"})
					break
				}
			}
		}
		for _, e := range snapshot.ProcessMap.Edges {
			e.ID = "process/" + e.ID
			e.From = "process/" + e.From
			e.To = "process/" + e.To
			edge(e)
		}
	}
	if view == "all" || view == "problems" {
		for _, p := range s.Store.RuleProblems("") {
			if p.Status != "active" {
				continue
			}
			id := "problem:" + p.ID
			node(model.Node{ID: id, Name: p.Name, Kind: "problem", Status: "warning"})
			edge(model.Edge{ID: "problem-service:" + p.ID, From: p.ServiceID, To: id, Kind: "threshold-evidence", Count: 1, Evidence: "explicit rule; not causal RCA"})
		}
	}
	if view == "all" || view == "kubernetes" || view == "ai" {
		for _, trace := range s.Store.APMTraces(from, to) {
			for _, sp := range trace.Spans {
				attrs := sp.ResourceAttributes
				get := func(key string) string { v, _ := attrs[key].(string); return v }
				if view == "all" || view == "kubernetes" {
					cluster := get("k8s.cluster.name")
					namespace := get("k8s.namespace.name")
					pod := get("k8s.pod.uid")
					if pod == "" {
						pod = get("k8s.pod.name")
					}
					if cluster != "" && namespace != "" && pod != "" {
						cid := "k8s:cluster:" + cluster
						nid := cid + "/namespace:" + namespace
						pid := nid + "/pod:" + pod
						node(model.Node{ID: cid, Name: cluster, Kind: "k8s.cluster", Status: "observed"})
						node(model.Node{ID: nid, Name: namespace, Kind: "k8s.namespace", Status: "observed"})
						podName := get("k8s.pod.name")
						if podName == "" {
							podName = pod
						}
						node(model.Node{ID: pid, Name: podName, Kind: "k8s.pod", Status: "observed"})
						edge(model.Edge{ID: nid, From: cid, To: nid, Kind: "contains", Count: 1, Evidence: "OTLP resource attributes"})
						edge(model.Edge{ID: pid, From: nid, To: pid, Kind: "contains", Count: 1, Evidence: "OTLP resource attributes"})
					}
				}
				if view == "all" || view == "ai" {
					system, _ := sp.Attributes["gen_ai.provider.name"].(string)
					if system == "" {
						system, _ = sp.Attributes["gen_ai.system"].(string)
					}
					if system != "" {
						id := "ai:" + url.QueryEscape(sp.ServiceNamespace) + ":" + url.QueryEscape(sp.ServiceName) + ":" + system
						node(model.Node{ID: id, Name: sp.ServiceName + " / " + system, Kind: "ai.service", Status: "observed"})
					}
				}
			}
		}
	}
	profiles := s.Store.Integrations()
	results := s.Store.IntegrationResults()
	for _, p := range profiles {
		if view != "all" && view != p.Kind && !(view == "devices" && (p.Kind == "snmp_exporter" || p.Kind == "snmp")) {
			continue
		}
		result, ok := results[p.ID]
		if !ok {
			continue
		}
		parent := "integration:" + p.ID
		state := "observed"
		if !p.Enabled {
			state = "paused"
		}
		if result.Status == "error" {
			state = "stale"
		}
		node(model.Node{ID: parent, Name: p.Name, Kind: p.Kind, Status: state})
		for _, v := range result.Resources {
			id := "resource:" + p.ID + ":" + v.ID
			node(model.Node{ID: id, Name: v.Name, Kind: v.Kind, Host: v.Location, Status: state})
			edge(model.Edge{ID: id, From: parent, To: id, Kind: "contains", Count: 1, Evidence: "provider inventory"})
			for _, iface := range v.Interfaces {
				iid := id + "/interface:" + iface.Index
				name := iface.Name
				if name == "" {
					name = iface.Index
				}
				node(model.Node{ID: iid, Name: name, Kind: "network.interface", Status: state})
				edge(model.Edge{ID: iid, From: id, To: iid, Kind: "contains", Count: 1, Evidence: "IF-MIB interface index"})
			}
		}
	}
	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].ID < graph.Nodes[j].ID })
	write(w, 200, map[string]any{"nodes": graph.Nodes, "edges": graph.Edges, "view": view, "truncated": truncated, "node_limit": 500, "edge_limit": 2000, "basis": fmt.Sprintf("observed inventory and retained OTLP; view=%s", view)})
}
