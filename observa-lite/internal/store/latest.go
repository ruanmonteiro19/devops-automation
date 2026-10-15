package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"
)

type LatestItem struct {
	ID        string    `json:"id"`
	Host      string    `json:"host"`
	HostID    string    `json:"host_id"`
	Group     string    `json:"group"`
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Unit      string    `json:"unit"`
	Source    string    `json:"source"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Change    string    `json:"change,omitempty"`
	Numeric   bool      `json:"numeric"`
	History   bool      `json:"history"`
}

func metricSeriesID(p model.MetricPoint) string {
	resource := p.ResourceAttributes
	service := p.ServiceName
	if deviceID, ok := resource["device.id"].(string); ok && deviceID != "" {
		resource = map[string]any{"device.id": deviceID}
		service = ""
	}
	b, _ := json.Marshal([]any{service, p.Name, resource, p.Attributes})
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:16])
}
func metricValue(p model.MetricPoint) (string, bool) {
	if p.Kind == "histogram" {
		count, _ := json.Marshal(p.Data["count"])
		sum, _ := json.Marshal(p.Data["sum"])
		return "Contagem: " + string(count) + " · Soma: " + string(sum), false
	}
	for _, key := range []string{"asInt", "asDouble"} {
		if value, ok := p.Data[key]; ok {
			switch v := value.(type) {
			case string:
				return v, true
			case float64:
				return strconv.FormatFloat(v, 'g', -1, 64), true
			case int64:
				return strconv.FormatInt(v, 10), true
			case json.Number:
				return string(v), true
			}
		}
	}
	b, _ := json.Marshal(p.Data)
	return string(b), false
}
func attribute(p model.MetricPoint, key string) string {
	v, _ := p.ResourceAttributes[key].(string)
	return v
}
func (s *Store) LatestData() []LatestItem {
	return s.LatestDataFiltered(func(LatestItem) bool { return true })
}
func (s *Store) LatestDataFiltered(accept func(LatestItem) bool) []LatestItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []LatestItem{}
	seen := map[string]int{}
	now := time.Now()
	for i := len(s.data.MetricPoints) - 1; i >= 0; i-- {
		p := s.data.MetricPoints[i]
		if now.Sub(p.Timestamp) > SignalRetention {
			continue
		}
		id := metricSeriesID(p)
		value, numeric := metricValue(p)
		if index, ok := seen[id]; ok {
			if index < 0 {
				continue
			}
			row := &out[index]
			if !p.Timestamp.Before(row.Timestamp) {
				continue
			}
			if row.Change == "" && row.Numeric && numeric {
				a, okA := new(big.Rat).SetString(row.Value)
				b, okB := new(big.Rat).SetString(value)
				if okA && okB {
					delta := new(big.Rat).Sub(a, b).FloatString(6)
					row.Change = strings.TrimRight(strings.TrimRight(delta, "0"), ".")
					if row.Change == "" || row.Change == "-0" {
						row.Change = "0"
					}
				}
			}
			continue
		}

		host := attribute(p, "host.name")
		if host == "" {
			host = p.ServiceName
		}
		hostID := attribute(p, "device.id")
		if hostID == "" {
			hostID = attribute(p, "host.id")
		}
		group := attribute(p, "host.group")
		source := "OTLP"
		status := "Disponível"
		reason := ""
		staleAfter := 10 * time.Minute
		if strings.HasPrefix(p.Name, "network.snmp.") {
			source = "SNMP"
			if profile, ok := s.data.Integrations[hostID]; ok {
				host = profile.Name
				group = profile.HostGroup
				if profile.IntervalSeconds > 300 {
					staleAfter = time.Duration(profile.IntervalSeconds*2) * time.Second
				}
				if !profile.Enabled {
					status = "Pausado"
				}
				if result, ok := s.data.IntegrationResults[hostID]; ok && result.Status == "error" {
					status = "Falha de coleta"
					reason = result.Error
				}
			}
		}
		if status == "Disponível" && now.Sub(p.Timestamp) > staleAfter {
			status = "Desatualizado"
		}
		seen[id] = -1
		row := LatestItem{ID: id, Host: host, HostID: hostID, Group: group, Name: p.Name, Key: p.Name, Value: value, Unit: p.Unit, Source: source, Status: status, Error: reason, Timestamp: p.Timestamp, Numeric: numeric, History: true}
		if accept(row) && len(out) < 5000 {
			seen[id] = len(out)
			out = append(out, row)
		}
	}
	for _, rawHost := range s.data.Hosts {
		h := s.projectInventory(rawHost)
		for _, process := range h.Processes {
			if model.IsSystemProcess(process) {
				continue
			}
			id := fmt.Sprintf("process:%s:%d", h.ID, process.PID)
			status := "Disponível"
			if now.Sub(h.LastSeen) > 3*time.Minute {
				status = "Desatualizado"
			}
			row := LatestItem{ID: id, Host: h.Name, HostID: h.ID, Group: h.HostGroup, Name: process.Service + " · Memória RSS", Key: fmt.Sprintf("process.rss[%d]", process.PID), Value: strconv.FormatUint(process.RSSBytes, 10), Unit: "B", Source: "Agente", Status: status, Timestamp: h.LastSeen, Numeric: true}
			if accept(row) && len(out) < 5000 {
				out = append(out, row)
			}
		}
	}
	for _, monitor := range s.data.Templates {
		status := "Disponível"
		if !monitor.Enabled {
			status = "Pausado"
		} else if monitor.LastRun.IsZero() {
			status = "Sem dados"
		} else if monitor.Status != "ok" {
			status = "Falha de coleta"
		}
		row := LatestItem{ID: "monitor:" + monitor.ID, Host: monitor.Target, HostID: monitor.ID, Name: monitor.Name, Key: monitor.Kind + ".response.time", Value: strconv.FormatFloat(monitor.LastValue, 'g', -1, 64), Unit: "ms", Source: "Monitor", Status: status, Error: monitor.LastError, Timestamp: monitor.LastRun, Numeric: true}
		if accept(row) && len(out) < 5000 {
			out = append(out, row)
		}
	}
	for _, profile := range s.data.Integrations {
		if profile.Kind != "snmp" {
			continue
		}
		template, ok := s.data.NetworkTemplates[profile.TemplateID]
		if !ok {
			continue
		}
		for _, item := range template.Items {
			if item.Supported {
				continue
			}
			row := LatestItem{ID: "unsupported:" + profile.ID + ":" + item.Key, Host: profile.Name, HostID: profile.ID, Group: profile.HostGroup, Name: item.Name, Key: item.Key, Value: "—", Source: "SNMP", Status: "Não suportado", Error: item.Reason}
			if accept(row) && len(out) < 5000 {
				out = append(out, row)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Host != out[j].Host {
			return out[i].Host < out[j].Host
		}
		return out[i].Key < out[j].Key
	})
	return out
}
func (s *Store) LatestHistory(id string, from, to time.Time) []LatestItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []LatestItem{}
	for _, p := range s.data.MetricPoints {
		if p.Timestamp.Before(from) || p.Timestamp.After(to) || metricSeriesID(p) != id {
			continue
		}
		value, numeric := metricValue(p)
		out = append(out, LatestItem{ID: id, Host: p.ServiceName, Name: p.Name, Key: p.Name, Value: value, Unit: p.Unit, Timestamp: p.Timestamp, Numeric: numeric})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.Before(out[j].Timestamp) })
	if len(out) > 5000 {
		out = out[len(out)-5000:]
	}
	return out
}
