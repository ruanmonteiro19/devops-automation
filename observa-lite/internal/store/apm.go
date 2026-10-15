package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"
)

const MaxStoredSpans = 20000
const SpanRetention = 24 * time.Hour
const MaxStoredSpanBytes = 32 << 20
const MaxTimeseriesServices = 200

func validID(v string, n int) bool {
	b, e := hex.DecodeString(v)
	if e != nil || len(b) != n {
		return false
	}
	for _, x := range b {
		if x != 0 {
			return true
		}
	}
	return false
}
func serviceID(v model.Span) string {
	return "svc:" + url.QueryEscape(v.ServiceNamespace) + ":" + url.QueryEscape(v.ServiceName)
}
func spanKey(v model.Span) string { return v.TraceID + ":" + v.SpanID }
func inWindow(v model.Span, from, to time.Time) bool {
	return !v.StartTime.Before(time.Now().Add(-SpanRetention)) && !v.StartTime.Before(from) && !v.StartTime.After(to)
}
func normalizeWindow(from, to time.Time) (time.Time, time.Time) {
	if to.IsZero() {
		to = time.Now().UTC()
	}
	if from.IsZero() {
		from = to.Add(-time.Hour)
	}
	return from, to
}

// IngestSpans validates the entire batch, copies caller-owned data and commits
// memory only after the atomic on-disk replacement succeeds.
func (s *Store) IngestSpans(input []model.Span) error {
	if len(input) > 10000 {
		return fmt.Errorf("span batch exceeds 10000")
	}
	b, e := json.Marshal(input)
	if e != nil {
		return e
	}
	if len(b) > 32<<20 {
		return fmt.Errorf("span batch too large")
	}
	var batch []model.Span
	if e = json.Unmarshal(b, &batch); e != nil {
		return e
	}
	now := time.Now().UTC()
	cutoff := now.Add(-SpanRetention)
	for i := range batch {
		v := &batch[i]
		encoded, _ := json.Marshal(v)
		if len(encoded) > 64<<10 {
			return fmt.Errorf("individual span exceeds 64 KiB")
		}
		v.TraceID = strings.ToLower(v.TraceID)
		v.SpanID = strings.ToLower(v.SpanID)
		v.ParentSpanID = strings.ToLower(v.ParentSpanID)
		if !validID(v.TraceID, 16) || !validID(v.SpanID, 8) || (v.ParentSpanID != "" && !validID(v.ParentSpanID, 8)) || v.ParentSpanID == v.SpanID {
			return fmt.Errorf("invalid span identifiers")
		}
		if strings.TrimSpace(v.ServiceName) == "" || len(v.ServiceName) > 512 || len(v.Name) > 4096 || v.StartTime.IsZero() || v.EndTime.Before(v.StartTime) || v.StartTime.After(now.Add(5*time.Minute)) || v.EndTime.After(now.Add(5*time.Minute)) {
			return fmt.Errorf("invalid span service or timestamps")
		}
		for j := range v.Links {
			link := &v.Links[j]
			link.TraceID, link.SpanID = strings.ToLower(link.TraceID), strings.ToLower(link.SpanID)
			if !validID(link.TraceID, 16) || !validID(link.SpanID, 8) {
				return fmt.Errorf("invalid span link identifiers")
			}
		}
		v.Kind = strings.ToLower(v.Kind)
		v.Status = strings.ToLower(v.Status)
		if v.ServiceNamespace == "" {
			v.ServiceNamespace, _ = v.ResourceAttributes["service.namespace"].(string)
		}
		if v.ServiceInstanceID == "" {
			v.ServiceInstanceID, _ = v.ResourceAttributes["service.instance.id"].(string)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	index := map[string]model.Span{}
	for _, v := range s.data.Spans {
		if !v.StartTime.Before(cutoff) {
			index[spanKey(v)] = v
		}
	}
	for _, v := range batch {
		if !v.StartTime.Before(cutoff) {
			if _, exists := index[spanKey(v)]; !exists {
				index[spanKey(v)] = v
			}
		}
	}
	next := make([]model.Span, 0, len(index))
	for _, v := range index {
		next = append(next, v)
	}
	sort.Slice(next, func(i, j int) bool {
		if next[i].StartTime.Equal(next[j].StartTime) {
			return spanKey(next[i]) < spanKey(next[j])
		}
		return next[i].StartTime.Before(next[j].StartTime)
	})
	if len(next) > MaxStoredSpans {
		next = next[len(next)-MaxStoredSpans:]
	}
	// Bound persisted payload as well as cardinality; retain newest spans.
	total := 0
	first := len(next)
	for i := len(next) - 1; i >= 0; i-- {
		encoded, _ := json.Marshal(next[i])
		if total+len(encoded) > MaxStoredSpanBytes {
			break
		}
		total += len(encoded)
		first = i
	}
	next = next[first:]
	old := s.data.Spans
	s.data.Spans = next
	if e = s.persist(); e != nil {
		s.data.Spans = old
		return e
	}
	return nil
}
func isRequest(v model.Span) bool { return v.Kind == "server" || v.Kind == "consumer" }
func SpanFailed(v model.Span) bool {
	if strings.EqualFold(v.Status, "error") {
		return true
	}
	code := v.Attributes["http.response.status_code"]
	if code == nil {
		code = v.Attributes["http.status_code"]
	}
	switch x := code.(type) {
	case float64:
		return x >= 500
	case int:
		return x >= 500
	case string:
		return len(x) == 3 && x[0] == '5'
	}
	return false
}
func metrics(spans []model.Span, seconds float64) model.APMMetrics {
	m := model.APMMetrics{}
	dur := []float64{}
	for _, v := range spans {
		if !isRequest(v) {
			continue
		}
		m.Requests++
		if SpanFailed(v) {
			m.Errors++
		}
		dur = append(dur, float64(v.EndTime.Sub(v.StartTime))/float64(time.Millisecond))
	}
	sort.Float64s(dur)
	if m.Requests > 0 {
		m.ErrorRate = float64(m.Errors) / float64(m.Requests) * 100
		m.P50MS = percentile(dur, .5)
		m.P95MS = percentile(dur, .95)
		m.P99MS = percentile(dur, .99)
	}
	if seconds > 0 {
		m.RequestsPerSecond = float64(m.Requests) / seconds
	}
	return m
}
func percentile(v []float64, p float64) float64 {
	if len(v) == 0 {
		return 0
	}
	return v[int(math.Ceil(float64(len(v))*p))-1]
}
func (s *Store) APMServices(from, to time.Time) []model.APMService {
	from, to = normalizeWindow(from, to)
	s.mu.RLock()
	defer s.mu.RUnlock()
	groups := map[string][]model.Span{}
	for _, v := range s.data.Spans {
		if inWindow(v, from, to) {
			groups[serviceID(v)] = append(groups[serviceID(v)], v)
		}
	}
	out := []model.APMService{}
	for id, spans := range groups {
		v := model.APMService{ID: id, Name: spans[0].ServiceName, Namespace: spans[0].ServiceNamespace, SpanCount: len(spans), APMMetrics: metrics(spans, to.Sub(from).Seconds()), Endpoints: []model.APMEndpoint{}}
		instances := map[string]bool{}
		endpoints := map[string][]model.Span{}
		for _, sp := range spans {
			if sp.ServiceInstanceID != "" {
				instances[sp.ServiceInstanceID] = true
			}
			if isRequest(sp) {
				name := sp.Name
				if route, ok := sp.Attributes["http.route"].(string); ok && route != "" {
					name = route
				}
				endpoints[name] = append(endpoints[name], sp)
			}
		}
		v.Instances = len(instances)
		for name, ss := range endpoints {
			v.Endpoints = append(v.Endpoints, model.APMEndpoint{Name: name, APMMetrics: metrics(ss, to.Sub(from).Seconds())})
		}
		sort.Slice(v.Endpoints, func(i, j int) bool { return v.Endpoints[i].Name < v.Endpoints[j].Name })
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (s *Store) APMTrace(id string) (model.APMTrace, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return buildTrace(id, s.data.Spans)
}
func buildTrace(id string, spans []model.Span) (model.APMTrace, bool) {
	t := model.APMTrace{TraceID: id, Spans: []model.Span{}, MissingParentIDs: []string{}, Completeness: "unknown: sampling and external spans cannot be verified"}
	ids := map[string]bool{}
	for _, v := range spans {
		if v.TraceID == id && !v.StartTime.Before(time.Now().Add(-SpanRetention)) {
			t.Spans = append(t.Spans, v)
			ids[v.SpanID] = true
			if t.StartTime.IsZero() || v.StartTime.Before(t.StartTime) {
				t.StartTime = v.StartTime
			}
			if v.EndTime.After(t.EndTime) {
				t.EndTime = v.EndTime
			}
		}
	}
	if len(t.Spans) == 0 {
		return t, false
	}
	missing := map[string]bool{}
	root := false
	for _, v := range t.Spans {
		if v.ParentSpanID == "" {
			root = true
		} else if !ids[v.ParentSpanID] {
			missing[v.ParentSpanID] = true
		}
	}
	for p := range missing {
		t.MissingParentIDs = append(t.MissingParentIDs, p)
	}
	sort.Strings(t.MissingParentIDs)
	t.Incomplete = !root || len(missing) > 0
	if t.Incomplete {
		t.Completeness = "incomplete: missing root or parent spans"
	}
	t.DurationMS = float64(t.EndTime.Sub(t.StartTime)) / float64(time.Millisecond)
	sort.Slice(t.Spans, func(i, j int) bool {
		if t.Spans[i].StartTime.Equal(t.Spans[j].StartTime) {
			return t.Spans[i].SpanID < t.Spans[j].SpanID
		}
		return t.Spans[i].StartTime.Before(t.Spans[j].StartTime)
	})
	b, _ := json.Marshal(t)
	var detached model.APMTrace
	json.Unmarshal(b, &detached)
	return detached, true
}
func (s *Store) APMTraces(from, to time.Time) []model.APMTrace {
	from, to = normalizeWindow(from, to)
	s.mu.RLock()
	defer s.mu.RUnlock()
	groups := map[string][]model.Span{}
	selected := map[string]bool{}
	cutoff := time.Now().Add(-SpanRetention)
	for _, v := range s.data.Spans {
		if !v.StartTime.Before(cutoff) {
			groups[v.TraceID] = append(groups[v.TraceID], v)
			if inWindow(v, from, to) {
				selected[v.TraceID] = true
			}
		}
	}
	out := []model.APMTrace{}
	for id := range selected {
		if t, ok := buildTrace(id, groups[id]); ok {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartTime.After(out[j].StartTime) })
	return out
}
func (s *Store) APMMap(from, to time.Time) model.Map {
	from, to = normalizeWindow(from, to)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return apmMap(s.data.Spans, from, to)
}
func apmMap(spans []model.Span, from, to time.Time) model.Map {
	out := model.Map{Nodes: []model.Node{}, Edges: []model.Edge{}}
	nodes := map[string]model.Node{}
	all := map[string]model.Span{}
	edges := map[string]model.Edge{}
	for _, v := range spans {
		all[spanKey(v)] = v
		if inWindow(v, from, to) {
			id := serviceID(v)
			n := nodes[id]
			n.ID = id
			n.Name = v.ServiceName
			n.Kind = "service"
			n.Status = "unknown"
			nodes[id] = n
		}
	}
	for _, v := range spans {
		if !inWindow(v, from, to) || v.ParentSpanID == "" {
			continue
		}
		p, ok := all[v.TraceID+":"+v.ParentSpanID]
		if !ok || p.StartTime.Before(time.Now().Add(-SpanRetention)) {
			continue
		}
		a, b := serviceID(p), serviceID(v)
		if _, exists := nodes[a]; !exists {
			nodes[a] = model.Node{ID: a, Name: p.ServiceName, Kind: "service", Status: "unknown"}
		}
		if a == b {
			continue
		}
		id := a + "->" + b
		e := edges[id]
		e.ID = id
		e.From = a
		e.To = b
		e.Kind = "span-parent"
		e.Count++
		e.Evidence = "trace_id + parent_span_id"
		e.Confidence = "observed"
		edges[id] = e
	}
	// A retained monitored server/consumer child proves the remote service;
	// do not also label that same call as an unmonitored external dependency.
	paired := map[string]bool{}
	for _, v := range spans {
		if isRequest(v) && v.ParentSpanID != "" && !v.StartTime.Before(time.Now().Add(-SpanRetention)) {
			if p, ok := all[v.TraceID+":"+v.ParentSpanID]; ok && serviceID(p) != serviceID(v) {
				paired[spanKey(p)] = true
			}
		}
	}
	for _, v := range spans {
		if !inWindow(v, from, to) || (v.Kind != "client" && v.Kind != "producer") || paired[spanKey(v)] {
			continue
		}
		n, evidence, ok := dependencyNode(v)
		if !ok {
			continue
		}
		nodes[n.ID] = n
		source := serviceID(v)
		key := source + "->" + n.ID
		edge := edges[key]
		edge.ID = key
		edge.From = source
		edge.To = n.ID
		edge.Kind = "observed-call"
		edge.Count++
		edge.Confidence = "observed"
		if edge.Evidence == "" {
			edge.Evidence = evidence + "; trace_id=" + v.TraceID + "; span_id=" + v.SpanID
		}
		edges[key] = edge
	}
	for _, n := range nodes {
		out.Nodes = append(out.Nodes, n)
	}
	for _, e := range edges {
		out.Edges = append(out.Edges, e)
	}
	sort.Slice(out.Nodes, func(i, j int) bool { return out.Nodes[i].ID < out.Nodes[j].ID })
	sort.Slice(out.Edges, func(i, j int) bool { return out.Edges[i].ID < out.Edges[j].ID })
	return out
}

// APMTimeseries returns at most 60 buckets for each observed service. A span is
// assigned by start time once, and only server/consumer spans count as requests.
func (s *Store) APMTimeseries(from, to time.Time) []model.APMServiceTimeseries {
	from, to = normalizeWindow(from, to)
	out := []model.APMServiceTimeseries{}
	if !to.After(from) {
		return out
	}
	const count = 60
	width := to.Sub(from) / count
	if width < time.Nanosecond {
		width = time.Nanosecond
	}
	bucketCount := count
	if to.Sub(from) < count*time.Nanosecond {
		bucketCount = int(to.Sub(from))
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	groups := map[string][]model.Span{}
	requests := map[string]int{}
	for _, v := range s.data.Spans {
		if inWindow(v, from, to) {
			id := serviceID(v)
			groups[id] = append(groups[id], v)
			if isRequest(v) {
				requests[id]++
			}
		}
	}
	ids := make([]string, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if requests[ids[i]] == requests[ids[j]] {
			return ids[i] < ids[j]
		}
		return requests[ids[i]] > requests[ids[j]]
	})
	if len(ids) > MaxTimeseriesServices {
		ids = ids[:MaxTimeseriesServices]
	}
	for _, id := range ids {
		samples := groups[id]
		buckets := make([][]model.Span, bucketCount)
		for _, v := range samples {
			index := int(v.StartTime.Sub(from) / width)
			if index >= bucketCount {
				index = bucketCount - 1
			}
			if index >= 0 {
				buckets[index] = append(buckets[index], v)
			}
		}
		meta := samples[0]
		series := model.APMServiceTimeseries{ServiceID: id, Name: meta.ServiceName, Namespace: meta.ServiceNamespace, Buckets: []model.APMBucket{}}
		for i, spans := range buckets {
			start := from.Add(time.Duration(i) * width)
			end := start.Add(width)
			if i == bucketCount-1 {
				end = to
			}
			series.Buckets = append(series.Buckets, model.APMBucket{StartTime: start, EndTime: end, APMMetrics: metrics(spans, end.Sub(start).Seconds())})
		}
		out = append(out, series)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ServiceID < out[j].ServiceID })
	return out
}

// dependencyNode identifies a logical dependency only from explicit semantic
// attributes. Its identity does not claim a physical database instance.
func dependencyNode(v model.Span) (model.Node, string, bool) {
	kind, system, logical, evidence := "", "", "", ""
	address := safePeer(analysisAttr(v.Attributes, "server.address", "net.peer.name", "http.host"))
	if address == "" {
		address = safePeer(analysisAttr(v.Attributes, "url.full", "http.url"))
	}
	port := analysisAttr(v.Attributes, "server.port", "net.peer.port")
	if system = analysisAttr(v.Attributes, "db.system.name", "db.system"); system != "" {
		kind = "database"
		logical = analysisAttr(v.Attributes, "db.namespace", "db.name")
		evidence = "OTLP explicit db.system/db.namespace; logical database dependency"
	} else if system = analysisAttr(v.Attributes, "messaging.system"); system != "" {
		kind = "messaging"
		logical = analysisAttr(v.Attributes, "messaging.destination.name", "messaging.destination")
		evidence = "OTLP explicit messaging.system/destination; logical messaging dependency"
	} else if address != "" && v.Kind == "client" {
		kind = "external"
		evidence = "OTLP explicit server.address/net.peer.name/URL hostname"
	} else {
		return model.Node{}, "", false
	}
	identity, _ := json.Marshal([]string{kind, v.ServiceNamespace, system, logical, address, port})
	hash := sha256.Sum256(identity)
	name := logical
	if name == "" {
		name = system
	}
	if kind == "external" {
		name = address
	} else if address != "" {
		name += " @ " + address
	}
	return model.Node{ID: "dep:" + kind + ":" + hex.EncodeToString(hash[:16]), Name: name, Kind: kind, Runtime: system, Status: "unknown"}, evidence, true
}
