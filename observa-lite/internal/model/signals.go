package model

import "time"

type LogRecord struct {
	Timestamp          time.Time      `json:"timestamp"`
	ObservedTimestamp  time.Time      `json:"observed_timestamp,omitempty"`
	ServiceName        string         `json:"service_name"`
	TraceID            string         `json:"trace_id,omitempty"`
	SpanID             string         `json:"span_id,omitempty"`
	SeverityText       string         `json:"severity_text,omitempty"`
	SeverityNumber     int32          `json:"severity_number"`
	Body               any            `json:"body"`
	Attributes         map[string]any `json:"attributes,omitempty"`
	ResourceAttributes map[string]any `json:"resource_attributes,omitempty"`
}

// MetricPoint preserves the OTLP value representation. Data contains asInt/asDouble
// for numeric points or count/sum/bucketCounts/explicitBounds/min/max for histograms.
// Integer strings remain exact; histogram quantiles are never synthesized.
type MetricPoint struct {
	Name               string         `json:"name"`
	Description        string         `json:"description,omitempty"`
	Unit               string         `json:"unit,omitempty"`
	Kind               string         `json:"kind"`
	Temporality        string         `json:"temporality,omitempty"`
	Monotonic          bool           `json:"monotonic"`
	Timestamp          time.Time      `json:"timestamp"`
	StartTime          time.Time      `json:"start_time,omitempty"`
	ServiceName        string         `json:"service_name"`
	Attributes         map[string]any `json:"attributes,omitempty"`
	ResourceAttributes map[string]any `json:"resource_attributes,omitempty"`
	Data               map[string]any `json:"data"`
}
