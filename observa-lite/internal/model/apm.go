package model

import "time"

type SpanEvent struct {
	Name       string         `json:"name"`
	Time       time.Time      `json:"time"`
	Attributes map[string]any `json:"attributes,omitempty"`
}
type SpanLink struct {
	TraceID                string         `json:"trace_id"`
	SpanID                 string         `json:"span_id"`
	TraceState             string         `json:"trace_state,omitempty"`
	Flags                  uint32         `json:"flags,omitempty"`
	Attributes             map[string]any `json:"attributes,omitempty"`
	DroppedAttributesCount uint32         `json:"dropped_attributes_count,omitempty"`
}
type InstrumentationScope struct {
	Name       string         `json:"name,omitempty"`
	Version    string         `json:"version,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}
type Span struct {
	Scope              InstrumentationScope `json:"scope"`
	ScopeSchemaURL     string               `json:"scope_schema_url,omitempty"`
	ResourceSchemaURL  string               `json:"resource_schema_url,omitempty"`
	TraceState         string               `json:"trace_state,omitempty"`
	Flags              uint32               `json:"flags,omitempty"`
	Links              []SpanLink           `json:"links,omitempty"`
	TraceID            string               `json:"trace_id"`
	SpanID             string               `json:"span_id"`
	ParentSpanID       string               `json:"parent_span_id,omitempty"`
	Name               string               `json:"name"`
	ServiceName        string               `json:"service_name"`
	ServiceNamespace   string               `json:"service_namespace,omitempty"`
	ServiceInstanceID  string               `json:"service_instance_id,omitempty"`
	Kind               string               `json:"kind"`
	Status             string               `json:"status"`
	StatusMessage      string               `json:"status_message,omitempty"`
	StartTime          time.Time            `json:"start_time"`
	EndTime            time.Time            `json:"end_time"`
	Attributes         map[string]any       `json:"attributes,omitempty"`
	ResourceAttributes map[string]any       `json:"resource_attributes,omitempty"`
	Events             []SpanEvent          `json:"events,omitempty"`
}
type APMMetrics struct {
	Requests          int     `json:"requests"`
	Errors            int     `json:"errors"`
	ErrorRate         float64 `json:"error_rate"`
	RequestsPerSecond float64 `json:"requests_per_second"`
	P50MS             float64 `json:"p50_ms"`
	P95MS             float64 `json:"p95_ms"`
	P99MS             float64 `json:"p99_ms"`
}
type APMEndpoint struct {
	Name string `json:"name"`
	APMMetrics
}
type APMService struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Instances int    `json:"instances"`
	SpanCount int    `json:"span_count"`
	APMMetrics
	Endpoints []APMEndpoint `json:"endpoints"`
}
type APMTrace struct {
	TraceID          string    `json:"trace_id"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	DurationMS       float64   `json:"duration_ms"`
	Spans            []Span    `json:"spans"`
	Incomplete       bool      `json:"incomplete"`
	Completeness     string    `json:"completeness"`
	MissingParentIDs []string  `json:"missing_parent_ids"`
}

// APMTimeseries uses half-open buckets [start,end), except the final bucket
// includes the query end. Empty buckets are retained for honest zero traffic.
type APMBucket struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	APMMetrics
}
type APMServiceTimeseries struct {
	ServiceID string      `json:"service_id"`
	Name      string      `json:"name"`
	Namespace string      `json:"namespace"`
	Buckets   []APMBucket `json:"buckets"`
}
