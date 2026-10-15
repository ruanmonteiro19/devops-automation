package store

import (
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ruanmonteiro19/observa-lite/internal/model"
)

const MaxAnalysisGroups = 200

func analysisAttr(attrs map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := attrs[key]; ok && v != nil {
			switch n := v.(type) {
			case string:
				if n != "" {
					return n
				}
			case json.Number:
				return n.String()
			case float64:
				return strconv.FormatFloat(n, 'f', -1, 64)
			case int:
				return strconv.Itoa(n)
			case int64:
				return strconv.FormatInt(n, 10)
			case bool:
				return strconv.FormatBool(n)
			}
		}
	}
	return ""
}
func analysisResource(v model.Span, keys ...string) string {
	if s := analysisAttr(v.ResourceAttributes, keys...); s != "" {
		return s
	}
	return analysisAttr(v.Attributes, keys...)
}
func safePeer(v string) string {
	if strings.Contains(v, "://") {
		if u, e := url.Parse(v); e == nil {
			return u.Hostname()
		}
		return ""
	}
	// A peer is an address, never credentials, path, query, or fragment.
	v = strings.SplitN(v, "/", 2)[0]
	v = strings.SplitN(v, "?", 2)[0]
	v = strings.SplitN(v, "#", 2)[0]
	if strings.Contains(v, "@") {
		return ""
	}
	return v
}
func groupAnalysis(groups map[string]*analysisAccumulator, name string, attrs map[string]string, v model.Span) {
	b, _ := json.Marshal(attrs)
	key := string(b)
	g := groups[key]
	if g == nil {
		g = &analysisAccumulator{name: name, attrs: attrs, traces: map[string]bool{}}
		groups[key] = g
	}
	g.count++
	if SpanFailed(v) {
		g.errors++
	}
	ms := float64(v.EndTime.Sub(v.StartTime)) / float64(time.Millisecond)
	g.durations = append(g.durations, ms)
	g.total += ms
	if len(g.traces) < 5 {
		g.traces[v.TraceID] = true
	}
}

type analysisAccumulator struct {
	name          string
	attrs         map[string]string
	count, errors int
	durations     []float64
	total         float64
	traces        map[string]bool
}

func finishAnalysis(groups map[string]*analysisAccumulator) ([]model.APMAnalysisGroup, bool) {
	out := []model.APMAnalysisGroup{}
	for _, g := range groups {
		sort.Float64s(g.durations)
		traces := []string{}
		for id := range g.traces {
			traces = append(traces, id)
		}
		sort.Strings(traces)
		out = append(out, model.APMAnalysisGroup{Name: g.name, Attributes: g.attrs, APMCallMetrics: model.APMCallMetrics{Count: g.count, Errors: g.errors, P50MS: percentile(g.durations, .5), P95MS: percentile(g.durations, .95), P99MS: percentile(g.durations, .99), TotalDurationMS: g.total, MeanDurationMS: g.total / float64(g.count), ExampleTraceIDs: traces}})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			a, _ := json.Marshal(out[i].Attributes)
			b, _ := json.Marshal(out[j].Attributes)
			return string(a) < string(b)
		}
		return out[i].Count > out[j].Count
	})
	truncated := len(out) > MaxAnalysisGroups
	if truncated {
		out = out[:MaxAnalysisGroups]
	}
	return out, truncated
}

// APMAnalysis groups only retained observed spans. It does not infer causality,
// database instances, or network dependencies from coincident errors.
func (s *Store) APMAnalysis(id string, from, to time.Time) model.APMAnalysis {
	from, to = normalizeWindow(from, to)
	result := model.APMAnalysis{ServiceID: id, From: from, To: to, CountUnit: "observed spans per category; categories can overlap", DurationMeaning: "sum of span durations; nested calls can overlap", GroupLimit: MaxAnalysisGroups}
	failures, endpoints, db, outbound, messaging, instances, versions := map[string]*analysisAccumulator{}, map[string]*analysisAccumulator{}, map[string]*analysisAccumulator{}, map[string]*analysisAccumulator{}, map[string]*analysisAccumulator{}, map[string]*analysisAccumulator{}, map[string]*analysisAccumulator{}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.data.Spans {
		if serviceID(v) != id || !inWindow(v, from, to) {
			continue
		}
		result.ObservedSpans++
		endpoint := analysisAttr(v.Attributes, "http.route", "rpc.method")
		if endpoint == "" {
			endpoint = v.Name
		}
		if isRequest(v) {
			method := analysisAttr(v.Attributes, "http.request.method", "http.method")
			groupAnalysis(endpoints, endpoint, map[string]string{"endpoint": endpoint, "method": method, "kind": v.Kind}, v)
		}
		exceptions := []map[string]string{}
		for _, event := range v.Events {
			if event.Name == "exception" {
				exceptions = append(exceptions, map[string]string{"exception.type": analysisAttr(event.Attributes, "exception.type"), "exception.message": analysisAttr(event.Attributes, "exception.message")})
			}
		}
		if SpanFailed(v) || len(exceptions) > 0 {
			attrs := map[string]string{"endpoint": endpoint, "status": v.Status, "status_message": v.StatusMessage, "http.status_code": analysisAttr(v.Attributes, "http.response.status_code", "http.status_code"), "exception.type": analysisAttr(v.Attributes, "exception.type"), "exception.message": analysisAttr(v.Attributes, "exception.message")}
			// One failure observation per span; preserve the first recorded exception.
			if len(exceptions) > 0 {
				attrs["exception.type"] = exceptions[0]["exception.type"]
				attrs["exception.message"] = exceptions[0]["exception.message"]
			}
			name := attrs["exception.type"]
			if name == "" {
				name = attrs["http.status_code"]
			}
			if name == "" {
				name = "Span failure"
			}
			groupAnalysis(failures, name, attrs, v)
		}
		system := analysisAttr(v.Attributes, "db.system.name", "db.system")
		if system != "" {
			namespace := analysisAttr(v.Attributes, "db.namespace", "db.name")
			operation := analysisAttr(v.Attributes, "db.operation.name", "db.operation")
			groupAnalysis(db, system, map[string]string{"db.system": system, "db.namespace": namespace, "db.operation": operation, "kind": v.Kind}, v)
		}
		if v.Kind == "client" {
			peer := safePeer(analysisAttr(v.Attributes, "server.address", "net.peer.name", "http.host"))
			if peer == "" {
				if raw := analysisAttr(v.Attributes, "url.full", "http.url"); raw != "" {
					peer = safePeer(raw)
				}
			}
			if peer != "" {
				groupAnalysis(outbound, peer, map[string]string{"peer": peer, "port": analysisAttr(v.Attributes, "server.port", "net.peer.port"), "method": analysisAttr(v.Attributes, "http.request.method", "http.method"), "kind": v.Kind}, v)
			}
		}
		if ms := analysisAttr(v.Attributes, "messaging.system"); ms != "" {
			dest := analysisAttr(v.Attributes, "messaging.destination.name", "messaging.destination")
			op := analysisAttr(v.Attributes, "messaging.operation.name", "messaging.operation.type", "messaging.operation")
			groupAnalysis(messaging, ms, map[string]string{"messaging.system": ms, "destination": dest, "operation": op, "kind": v.Kind}, v)
		}
		instance := v.ServiceInstanceID
		if instance == "" {
			instance = analysisResource(v, "service.instance.id")
		}
		if instance != "" {
			groupAnalysis(instances, instance, map[string]string{"service.instance.id": instance}, v)
		}
		if version := analysisResource(v, "service.version"); version != "" {
			groupAnalysis(versions, version, map[string]string{"service.version": version}, v)
		}
	}
	var cut bool
	result.Failures, cut = finishAnalysis(failures)
	result.Truncated = result.Truncated || cut
	result.Endpoints, cut = finishAnalysis(endpoints)
	result.Truncated = result.Truncated || cut
	result.DatabaseCalls, cut = finishAnalysis(db)
	result.Truncated = result.Truncated || cut
	result.OutboundCalls, cut = finishAnalysis(outbound)
	result.Truncated = result.Truncated || cut
	result.Messaging, cut = finishAnalysis(messaging)
	result.Truncated = result.Truncated || cut
	result.Instances, cut = finishAnalysis(instances)
	result.Truncated = result.Truncated || cut
	result.Versions, cut = finishAnalysis(versions)
	result.Truncated = result.Truncated || cut
	return result
}
