package server

import (
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net"
	"net/url"
	"strings"
	"time"
)

// Links require an advertised LLDP neighbor, an observed socket, or explicit OTLP host/peer identity.
func (s *Server) topologyRelations(view string, from, to time.Time, snapshot model.Snapshot, node func(model.Node), edge func(model.Edge)) {
	if view != "all" && view != "devices" && view != "network" {
		return
	}
	results := s.Store.IntegrationResults()
	profiles := s.Store.Integrations()
	byIP := map[string]string{}
	byName := map[string]string{}
	hostByIP := map[string]model.Host{}
	hostByName := map[string]model.Host{}
	hostByID := map[string]model.Host{}
	linkedHosts := map[string]bool{}
	normalized := func(name string) string { return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), ".")) }
	addName := func(name, id string) {
		name = normalized(name)
		if name == "" {
			return
		}
		if prior, ok := byName[name]; ok && prior != id {
			byName[name] = ""
		} else {
			byName[name] = id
		}
	}
	for _, h := range snapshot.Hosts {
		hostByID[h.ID] = h
		name := normalized(h.Name)
		if old, ok := hostByName[name]; ok && old.ID != h.ID {
			hostByName[name] = model.Host{}
		} else {
			hostByName[name] = h
		}
		for _, ip := range h.Addresses {
			if parsed := net.ParseIP(ip); parsed != nil {
				key := parsed.String()
				if old, ok := hostByIP[key]; ok && old.ID != h.ID {
					hostByIP[key] = model.Host{}
				} else {
					hostByIP[key] = h
				}
			}
		}
	}
	addIP := func(ip, id string) {
		if old, ok := byIP[ip]; ok && old != id {
			byIP[ip] = ""
		} else {
			byIP[ip] = id
		}
	}
	for _, p := range profiles {
		if p.Kind != "snmp" && p.Kind != "snmp_exporter" {
			continue
		}
		for _, v := range results[p.ID].Resources {
			id := "resource:" + p.ID + ":" + v.ID
			if parsed := net.ParseIP(v.Attributes["address"]); parsed != nil {
				addIP(parsed.String(), id)
			}
			if parsed := net.ParseIP(p.Target); parsed != nil {
				addIP(parsed.String(), id)
			}
			addName(v.Name, id)
			addName(p.Name, id)
			addName(v.Attributes["sysName"], id)
		}
	}
	hostNode := func(h model.Host) string {
		id := "host:" + h.ID
		status := "observed"
		if time.Since(h.LastSeen) > 3*time.Minute {
			status = "stale"
		}
		node(model.Node{ID: id, Name: h.Name, Kind: "host", Status: status})
		linkedHosts[h.ID] = true
		return id
	}
	for _, p := range profiles {
		if p.Kind != "snmp" && p.Kind != "snmp_exporter" {
			continue
		}
		result := results[p.ID]
		for _, resource := range result.Resources {
			source := "resource:" + p.ID + ":" + resource.ID
			for _, neighbor := range resource.Neighbors {
				target := ""
				if neighbor.RemoteIP != "" {
					target = byIP[neighbor.RemoteIP]
					if target == "" {
						if h, ok := hostByIP[neighbor.RemoteIP]; ok && h.ID != "" {
							target = hostNode(h)
						}
					}
				}
				if target == "" && neighbor.DNSName != "" {
					target = byName[normalized(neighbor.DNSName)]
					if target == "" {
						if h, ok := hostByName[normalized(neighbor.DNSName)]; ok && h.ID != "" {
							target = hostNode(h)
						}
					}
				}
				if target == "" && neighbor.RemoteName != "" {
					target = byName[normalized(neighbor.RemoteName)]
					if target == "" {
						if h, ok := hostByName[normalized(neighbor.RemoteName)]; ok && h.ID != "" {
							target = hostNode(h)
						}
					}
				}
				if target == "" {
					identity := neighbor.ChassisID
					if identity == "" {
						identity = neighbor.RemoteIP
					}
					if identity == "" {
						identity = source + ":" + neighbor.LocalPort + ":" + neighbor.RemoteName
					}
					target = "lldp-neighbor:" + url.QueryEscape(identity)
					name := neighbor.DNSName
					if name == "" {
						name = neighbor.RemoteName
					}
					if name == "" {
						name = neighbor.RemoteIP
					}
					if name == "" {
						name = neighbor.ChassisID
					}
					if name == "" {
						name = "Vizinho LLDP"
					}
					node(model.Node{ID: target, Name: name, Kind: "network.neighbor", Status: "advertised"})
				}
				if target == source {
					continue
				}
				local := source
				if neighbor.LocalIfIndex != "" {
					local = source + "/interface:" + neighbor.LocalIfIndex
				} else {
					local = source + "/lldp-port:" + url.QueryEscape(neighbor.LocalPort)
					node(model.Node{ID: local, Name: neighbor.LocalPort, Kind: "network.interface", Status: "observed"})
					edge(model.Edge{ID: source + "/contains/" + local, From: source, To: local, Kind: "contains", Count: 1, Evidence: "LLDP local port identity; not assumed to be IF-MIB index"})
				}
				edge(model.Edge{ID: local + "/lldp/" + target, From: local, To: target, Kind: "lldp", Count: 1, Evidence: fmt.Sprintf("LLDP: porta local %s; porta remota %s; DNS %s; IP %s; nome anunciado %s", neighbor.LocalPort, neighbor.RemotePort, neighbor.DNSName, neighbor.RemoteIP, neighbor.RemoteName), Confidence: "advertised"})
			}
		}
	}
	for _, h := range snapshot.Hosts {
		for _, connection := range h.Connections {
			if connection.State != "ESTABLISHED" && connection.State != "SYN_SENT" && connection.State != "CLOSE_WAIT" {
				continue
			}
			ip := net.ParseIP(connection.RemoteIP)
			if ip == nil {
				continue
			}
			target := byIP[ip.String()]
			if target == "" {
				continue
			}
			source := hostNode(h)
			for _, process := range h.Processes {
				if process.PID != connection.PID || model.IsSystemProcess(process) || process.Service == "" {
					continue
				}
				source = "process/svc:" + h.ID + ":" + process.Service
				node(model.Node{ID: source, Name: process.Service, Kind: "process", Host: h.Name, Runtime: process.Runtime, Status: "observed"})
				edge(model.Edge{ID: "runs/" + source, From: "host:" + h.ID, To: source, Kind: "contains", Count: 1, Evidence: "agent inventory"})
				break
			}
			edge(model.Edge{ID: source + "/socket/" + target + fmt.Sprint(connection.RemotePort), From: source, To: target, Kind: "socket", Count: 1, Evidence: fmt.Sprintf("agent socket %s:%d (%s); not a physical switch port", connection.RemoteIP, connection.RemotePort, connection.State), Confidence: "observed"})
		}
	}
	for _, trace := range s.Store.APMTraces(from, to) {
		for _, span := range trace.Spans {
			serviceID := "svc:" + url.QueryEscape(span.ServiceNamespace) + ":" + url.QueryEscape(span.ServiceName)
			get := func(key string) string { v, _ := span.ResourceAttributes[key].(string); return v }
			h, matched := hostByID[get("host.id")]
			basis := "OTLP resource host.id"
			if !matched {
				h, matched = hostByName[normalized(get("host.name"))]
				basis = "OTLP resource host.name"
			}
			if matched && h.ID != "" && (view != "devices" || linkedHosts[h.ID]) {
				host := hostNode(h)
				node(model.Node{ID: serviceID, Name: span.ServiceName, Kind: "service", Host: h.Name, Status: "observed"})
				edge(model.Edge{ID: host + "/apm/" + serviceID, From: host, To: serviceID, Kind: "runs-service", Count: 1, Evidence: basis, Confidence: "resource-identity"})
			}
			for _, key := range []string{"server.address", "network.peer.address", "net.peer.ip"} {
				address, _ := span.Attributes[key].(string)
				if ip := net.ParseIP(address); ip != nil {
					if target := byIP[ip.String()]; target != "" {
						node(model.Node{ID: serviceID, Name: span.ServiceName, Kind: "service", Status: "observed"})
						edge(model.Edge{ID: serviceID + "/peer/" + target, From: serviceID, To: target, Kind: "span-peer", Count: 1, Evidence: "OTLP " + key + "=" + address, Confidence: "instrumented"})
					}
				}
			}
		}
	}
}
