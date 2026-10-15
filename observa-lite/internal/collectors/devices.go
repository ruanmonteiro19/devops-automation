package collectors

import (
	"context"
	"errors"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var promSample = regexp.MustCompile(`^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{(.*)\})?\s+([^\s]+)(?:\s+[^\s]+)?$`)
var promLabel = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)="((?:[^"\\]|\\.)*)"`)

func parseLabels(raw string) (map[string]string, error) {
	out := map[string]string{}
	for strings.TrimSpace(raw) != "" {
		raw = strings.TrimSpace(raw)
		m := promLabel.FindStringSubmatch(raw)
		if m == nil {
			return nil, errors.New("invalid metric labels")
		}
		v, e := strconv.Unquote(`"` + m[2] + `"`)
		if e != nil {
			return nil, errors.New("invalid metric label escaping")
		}
		out[m[1]] = v
		raw = strings.TrimSpace(raw[len(m[0]):])
		if raw == "" {
			break
		}
		if raw[0] != ',' {
			return nil, errors.New("invalid metric label delimiter")
		}
		raw = raw[1:]
	}
	return out, nil
}
func (r Runner) deviceInventory(ctx context.Context, p model.Integration) ([]model.IntegrationResource, bool, error) {
	ip := net.ParseIP(p.Target)
	if ip == nil || (!r.AllowPrivate && restricted(ip)) {
		return nil, false, errors.New("network target denied; enable OBSERVA_ALLOW_PRIVATE_TARGETS for trusted internal devices")
	}
	u, e := url.Parse(p.Endpoint)
	if e != nil {
		return nil, false, errors.New("invalid exporter URL")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/snmp"
	u.RawQuery = url.Values{"target": {p.Target}, "module": {p.Module}, "auth": {p.Auth}}.Encode()
	body, e := r.integrationRequest(ctx, "GET", u.String(), nil, http.Header{})
	if e != nil {
		return nil, false, e
	}
	return parseDeviceMetrics(string(body), p.Target)
}
func parseDeviceMetrics(body, target string) ([]model.IntegrationResource, bool, error) {
	resource := model.IntegrationResource{ID: target, Name: target, Kind: "network.device", Status: "observed", Attributes: map[string]string{"address": target}, Metrics: map[string]float64{}, Interfaces: []model.DeviceInterface{}}
	interfaces := map[string]*model.DeviceInterface{}
	truncated := false
	relevant := 0
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := promSample.FindStringSubmatch(line)
		if m == nil {
			return nil, false, errors.New("invalid Prometheus exposition from exporter")
		}
		labels, e := parseLabels(m[2])
		if e != nil {
			return nil, false, e
		}
		v, e := strconv.ParseFloat(m[3], 64)
		if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		name := m[1]
		if index := labels["ifIndex"]; index != "" && strings.HasPrefix(name, "if") {
			relevant++
			entry, exists := interfaces[index]
			if !exists {
				if len(interfaces) >= 256 {
					truncated = true
					continue
				}
				entry = &model.DeviceInterface{Index: index, Name: index, Metrics: map[string]float64{}}
				interfaces[index] = entry
			}
			if n := labels["ifName"]; n != "" {
				entry.Name = n
			}
			if desc := labels["ifDescr"]; desc != "" {
				entry.Description = desc
			}
			if alias := labels["ifAlias"]; alias != "" {
				entry.Description = alias
			}
			if len(entry.Metrics) < 64 {
				entry.Metrics[name] = v
			} else {
				truncated = true
			}
			continue
		}
		if strings.HasPrefix(name, "sys") || strings.Contains(strings.ToLower(name), "cpu") || strings.Contains(strings.ToLower(name), "memory") || strings.HasPrefix(name, "hrMemory") || strings.Contains(strings.ToLower(name), "processor") || strings.Contains(strings.ToLower(name), "temperature") || strings.Contains(strings.ToLower(name), "fan") || strings.HasPrefix(name, "hrStorage") {
			relevant++
			if len(resource.Metrics) < 200 {
				metricKey := name
				if len(labels) > 0 && !strings.HasPrefix(name, "sys") {
					keys := []string{}
					for k := range labels {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					parts := []string{}
					for _, k := range keys {
						parts = append(parts, k+"="+strconv.Quote(labels[k]))
					}
					metricKey += "{" + strings.Join(parts, ",") + "}"
				}
				resource.Metrics[metricKey] = v
			} else {
				truncated = true
			}
			for k, val := range labels {
				if strings.HasPrefix(k, "sys") && len(val) <= 4096 {
					resource.Attributes[k] = val
				}
			}
			if n := labels["sysName"]; n != "" {
				resource.Name = n
			}
		}
	}
	if relevant == 0 {
		return nil, false, errors.New("no supported system or IF-MIB data; check exporter modules and device permissions")
	}
	keys := []string{}
	for k := range interfaces {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		resource.Interfaces = append(resource.Interfaces, *interfaces[k])
	}
	return []model.IntegrationResource{resource}, truncated, nil
}

// DeviceRates derives traffic only from two successful, recent samples. Counter
// decreases/reboots and ambiguous wraps deliberately leave rate fields absent.
func DeviceRates(current *model.IntegrationResult, previous model.IntegrationResult) {
	seconds := current.SuccessAt.Sub(previous.SuccessAt).Seconds()
	if previous.SuccessAt.IsZero() || seconds < 1 || seconds > 1800 {
		return
	}
	old := map[string]model.IntegrationResource{}
	for _, v := range previous.Resources {
		old[v.ID] = v
	}
	for ri := range current.Resources {
		resource := &current.Resources[ri]
		prior, ok := old[resource.ID]
		if !ok {
			continue
		}
		reboot := false
		for _, key := range []string{"sysUpTime", "sysUpTimeInstance"} {
			a, hasA := resource.Metrics[key]
			b, hasB := prior.Metrics[key]
			if hasA && hasB && a < b {
				reboot = true
			}
		}
		if reboot {
			continue
		}
		interfaces := map[string]model.DeviceInterface{}
		for _, v := range prior.Interfaces {
			interfaces[v.Index] = v
		}
		for ii := range resource.Interfaces {
			iface := &resource.Interfaces[ii]
			before, ok := interfaces[iface.Index]
			if !ok {
				continue
			}
			for _, direction := range []string{"In", "Out"} {
				for _, counter := range []string{"ifHC" + direction + "Octets", "if" + direction + "Octets"} {
					value, has := iface.Metrics[counter]
					previousValue, had := before.Metrics[counter]
					if !has || !had {
						continue
					}
					delta := value - previousValue
					if delta >= 0 {
						bps := delta * 8 / seconds
						if math.IsNaN(bps) || math.IsInf(bps, 0) {
							break
						}
						name := "observa_" + strings.ToLower(direction) + "_bits_per_second"
						iface.Metrics[name] = bps
						speed := iface.Metrics["ifHighSpeed"] * 1000000
						if speed <= 0 {
							speed = iface.Metrics["ifSpeed"]
						}
						if speed > 0 {
							util := bps / speed * 100
							if !math.IsNaN(util) && !math.IsInf(util, 0) {
								iface.Metrics["observa_"+strings.ToLower(direction)+"_utilization_percent"] = util
							}
						}
					}
					break
				}
			}
		}
	}
}
