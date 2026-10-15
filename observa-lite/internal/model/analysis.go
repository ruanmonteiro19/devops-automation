package model

import "time"

// APMCallMetrics counts observed spans, not inbound requests. Nested spans can
// overlap, so total duration is a sum of span durations rather than wall time.
type APMCallMetrics struct {
	Count           int      `json:"count"`
	Errors          int      `json:"errors"`
	P50MS           float64  `json:"p50_ms"`
	P95MS           float64  `json:"p95_ms"`
	P99MS           float64  `json:"p99_ms"`
	TotalDurationMS float64  `json:"total_duration_ms"`
	MeanDurationMS  float64  `json:"mean_duration_ms"`
	ExampleTraceIDs []string `json:"example_trace_ids"`
}
type APMAnalysisGroup struct {
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
	APMCallMetrics
}
type APMAnalysis struct {
	ServiceID       string             `json:"service_id"`
	From            time.Time          `json:"from"`
	To              time.Time          `json:"to"`
	CountUnit       string             `json:"count_unit"`
	DurationMeaning string             `json:"duration_meaning"`
	ObservedSpans   int                `json:"observed_spans"`
	Failures        []APMAnalysisGroup `json:"failures"`
	Endpoints       []APMAnalysisGroup `json:"endpoints"`
	DatabaseCalls   []APMAnalysisGroup `json:"database_calls"`
	OutboundCalls   []APMAnalysisGroup `json:"outbound_calls"`
	Messaging       []APMAnalysisGroup `json:"messaging"`
	Instances       []APMAnalysisGroup `json:"instances"`
	Versions        []APMAnalysisGroup `json:"versions"`
	Truncated       bool               `json:"truncated"`
	GroupLimit      int                `json:"group_limit"`
}
