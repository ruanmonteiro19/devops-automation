package server

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	logscollector "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metricscollector "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// decodeSignal uses the official OTLP protobuf schemas. OTLP JSON's hexadecimal
// trace/span IDs are translated before the standard protobuf JSON decoder.
func decodeSignal(w http.ResponseWriter, r *http.Request, message proto.Message) (string, bool) {
	typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || (typ != "application/json" && typ != "application/x-protobuf") {
		http.Error(w, "unsupported OTLP content type", 415)
		return "", false
	}
	var reader io.Reader = http.MaxBytesReader(w, r.Body, otlpMaxBody)
	switch r.Header.Get("Content-Encoding") {
	case "", "identity":
	case "gzip":
		gz, e := gzip.NewReader(reader)
		if e != nil {
			http.Error(w, "invalid gzip body", 400)
			return "", false
		}
		defer gz.Close()
		reader = gz
	default:
		http.Error(w, "unsupported content encoding", 415)
		return "", false
	}
	body, err := io.ReadAll(io.LimitReader(reader, otlpMaxBody+1))
	if err != nil {
		otlpDecodeError(w, err)
		return "", false
	}
	if len(body) > otlpMaxBody {
		http.Error(w, "OTLP payload exceeds 4 MiB", 413)
		return "", false
	}
	if typ == "application/x-protobuf" {
		err = proto.Unmarshal(body, message)
	} else {
		if strings.TrimSpace(string(body)) == "null" {
			http.Error(w, "invalid OTLP JSON", 400)
			return "", false
		}
		body, err = signalJSONIDs(body)
		if err == nil {
			err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, message)
		}
	}
	if err != nil {
		http.Error(w, "invalid OTLP payload", 400)
		return "", false
	}
	return typ, true
}
func signalJSONIDs(body []byte) ([]byte, error) {
	var request map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&request); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	// Log record and exemplar identity fields use OTLP hex; bytes attributes keep base64.
	var walk func(any) error
	walk = func(node any) error {
		switch v := node.(type) {
		case []any:
			for _, item := range v {
				if err := walk(item); err != nil {
					return err
				}
			}
		case map[string]any:
			for key, item := range v {
				if key == "logRecords" || key == "log_records" || key == "exemplars" {
					list, ok := item.([]any)
					if !ok {
						return errors.New("invalid log records")
					}
					for _, entry := range list {
						record, ok := entry.(map[string]any)
						if !ok {
							return errors.New("invalid log record")
						}
						for _, id := range []struct {
							key string
							n   int
						}{{"traceId", 32}, {"trace_id", 32}, {"spanId", 16}, {"span_id", 16}} {
							if raw, ok := record[id.key]; ok {
								value, ok := raw.(string)
								if !ok {
									return errors.New("invalid ID")
								}
								if value != "" && !otlpID(value, id.n) {
									return errors.New("invalid log ID")
								}
								bytes, _ := hex.DecodeString(value)
								record[id.key] = base64.StdEncoding.EncodeToString(bytes)
							}
						}
					}
					continue
				}
				if err := walk(item); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(request); err != nil {
		return nil, err
	}
	return json.Marshal(request)
}
func signalAttributes(attrs []*commonpb.KeyValue) (map[string]any, error) {
	out := map[string]any{}
	for _, attr := range attrs {
		if attr == nil || attr.Key == "" || attr.Value == nil {
			return nil, errors.New("invalid signal attribute")
		}
		body, err := protojson.Marshal(attr.Value)
		if err != nil {
			return nil, err
		}
		var value map[string]json.RawMessage
		if err = json.Unmarshal(body, &value); err != nil {
			return nil, err
		}
		v, err := otlpValue(value, 0)
		if err != nil {
			return nil, err
		}
		out[attr.Key] = v
	}
	return out, nil
}
func signalService(attrs map[string]any) string {
	v, _ := attrs["service.name"].(string)
	if strings.TrimSpace(v) == "" {
		return "unknown_service"
	}
	return v
}
func signalTime(n uint64, optional bool) (time.Time, error) {
	if n == 0 && optional {
		return time.Time{}, nil
	}
	if n == 0 || n > math.MaxInt64 {
		return time.Time{}, errors.New("invalid signal timestamp")
	}
	t := time.Unix(0, int64(n)).UTC()
	if t.After(time.Now().Add(5 * time.Minute)) {
		return time.Time{}, errors.New("signal timestamp is in future")
	}
	return t, nil
}
func signalResponse(w http.ResponseWriter, typ string, err error) {
	if err != nil {
		var validation *store.SignalValidationError
		if errors.As(err, &validation) {
			http.Error(w, validation.Error(), 400)
		} else {
			http.Error(w, "signal storage failed", 500)
		}
		return
	}
	w.Header().Set("Content-Type", typ)
	w.WriteHeader(200)
	if typ == "application/json" {
		_, _ = w.Write([]byte("{}"))
	}
}
func (s *Server) ingestLogs(w http.ResponseWriter, r *http.Request) {
	var export logscollector.ExportLogsServiceRequest
	typ, ok := decodeSignal(w, r, &export)
	if !ok {
		return
	}
	batch, err := convertLogs(&export)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if len(batch) > 0 {
		err = s.Store.IngestLogs(batch)
	}
	signalResponse(w, typ, err)
}
func convertLogs(export *logscollector.ExportLogsServiceRequest) ([]model.LogRecord, error) {
	received := time.Now().UTC()
	out := []model.LogRecord{}
	for _, resource := range export.ResourceLogs {
		if resource == nil {
			return nil, errors.New("invalid resource logs")
		}
		var attrs []*commonpb.KeyValue
		if resource.Resource != nil {
			attrs = resource.Resource.Attributes
		}
		ra, err := signalAttributes(attrs)
		if err != nil {
			return nil, err
		}
		for _, scope := range resource.ScopeLogs {
			if scope == nil {
				return nil, errors.New("invalid scope logs")
			}
			for _, log := range scope.LogRecords {
				if log == nil {
					return nil, errors.New("invalid log")
				}
				if len(out) >= store.MaxLogs {
					return nil, errors.New("log batch exceeds 10000 records")
				}
				observed, err := signalTime(log.ObservedTimeUnixNano, true)
				if err != nil {
					return nil, err
				}
				if observed.IsZero() {
					observed = received
				}
				at, err := signalTime(log.TimeUnixNano, true)
				if err != nil {
					return nil, err
				}
				if at.IsZero() {
					at = observed
				}

				traceID, spanID := hex.EncodeToString(log.TraceId), hex.EncodeToString(log.SpanId)
				if (traceID != "" && !otlpID(traceID, 32)) || (spanID != "" && !otlpID(spanID, 16)) {
					return nil, errors.New("invalid log trace/span ID")
				}
				a, err := signalAttributes(log.Attributes)
				if err != nil {
					return nil, err
				}
				var body any
				if log.Body != nil && log.Body.Value != nil {
					encoded, err := protojson.Marshal(log.Body)
					if err != nil {
						return nil, err
					}
					var raw map[string]json.RawMessage
					if err = json.Unmarshal(encoded, &raw); err != nil {
						return nil, err
					}
					body, err = otlpValue(raw, 0)
					if err != nil {
						return nil, err
					}
				}
				if log.SeverityNumber < 0 || log.SeverityNumber > 24 {
					return nil, errors.New("invalid log severity")
				}
				out = append(out, model.LogRecord{Timestamp: at, ObservedTimestamp: observed, ServiceName: signalService(ra), TraceID: traceID, SpanID: spanID, SeverityText: log.SeverityText, SeverityNumber: int32(log.SeverityNumber), Body: body, Attributes: a, ResourceAttributes: ra})
			}
		}
	}
	return out, nil
}
func (s *Server) ingestMetrics(w http.ResponseWriter, r *http.Request) {
	var export metricscollector.ExportMetricsServiceRequest
	typ, ok := decodeSignal(w, r, &export)
	if !ok {
		return
	}
	batch, err := convertMetrics(&export)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if len(batch) > 0 {
		err = s.Store.IngestMetrics(batch)
	}
	signalResponse(w, typ, err)
}
func metricData(message proto.Message) (map[string]any, error) {
	b, err := protojson.Marshal(message)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = json.Unmarshal(b, &result)
	delete(result, "attributes")
	delete(result, "timeUnixNano")
	delete(result, "startTimeUnixNano")
	if histogram, ok := message.(*metricspb.HistogramDataPoint); ok {
		result["count"] = strconv.FormatUint(histogram.Count, 10)
	}
	if err != nil {
		return nil, err
	}
	if exemplars, ok := result["exemplars"].([]any); ok {
		for _, item := range exemplars {
			exemplar, ok := item.(map[string]any)
			if !ok {
				return nil, errors.New("invalid exemplar")
			}
			for _, id := range []struct {
				key string
				n   int
			}{{"traceId", 32}, {"spanId", 16}} {
				if value, ok := exemplar[id.key].(string); ok {
					decoded, e := base64.StdEncoding.DecodeString(value)
					if e != nil || !otlpID(hex.EncodeToString(decoded), id.n) {
						return nil, errors.New("invalid exemplar trace/span ID")
					}
					exemplar[id.key] = hex.EncodeToString(decoded)
				}
			}
		}
	}
	return result, nil
}
func temporality(t metricspb.AggregationTemporality) (string, error) {
	switch t {
	case metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA:
		return "delta", nil
	case metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE:
		return "cumulative", nil
	}
	return "", errors.New("metric requires delta or cumulative temporality")
}
func convertMetrics(export *metricscollector.ExportMetricsServiceRequest) ([]model.MetricPoint, error) {
	out := []model.MetricPoint{}
	for _, resource := range export.ResourceMetrics {
		if resource == nil {
			return nil, errors.New("invalid resource metrics")
		}
		var attributes []*commonpb.KeyValue
		if resource.Resource != nil {
			attributes = resource.Resource.Attributes
		}
		ra, err := signalAttributes(attributes)
		if err != nil {
			return nil, err
		}
		for _, scope := range resource.ScopeMetrics {
			if scope == nil {
				return nil, errors.New("invalid scope metrics")
			}
			for _, metric := range scope.Metrics {
				if metric == nil || strings.TrimSpace(metric.Name) == "" {
					return nil, errors.New("metric name is required")
				}
				base := model.MetricPoint{Name: metric.Name, Description: metric.Description, Unit: metric.Unit, ServiceName: signalService(ra), ResourceAttributes: ra}
				add := func(point proto.Message, start, end uint64, attributes []*commonpb.KeyValue) error {
					if len(out) >= store.MaxMetricPoints {
						return errors.New("metric batch exceeds 50000 points")
					}
					v := base
					var err error
					v.Timestamp, err = signalTime(end, false)
					if err != nil {
						return err
					}
					v.StartTime, err = signalTime(start, true)
					if err != nil {
						return err
					}
					if !v.StartTime.IsZero() && v.StartTime.After(v.Timestamp) {
						return errors.New("metric start is after end")
					}
					v.Attributes, err = signalAttributes(attributes)
					if err != nil {
						return err
					}
					v.Data, err = metricData(point)
					if err != nil {
						return err
					}
					out = append(out, v)
					return nil
				}
				numbers := func(points []*metricspb.NumberDataPoint) error {
					for _, point := range points {
						if point == nil || point.Value == nil {
							return errors.New("numeric metric value is required")
						}
						if v, ok := point.Value.(*metricspb.NumberDataPoint_AsDouble); ok && (math.IsNaN(v.AsDouble) || math.IsInf(v.AsDouble, 0)) {
							return errors.New("nonfinite metric value")
						}
						if err := add(point, point.StartTimeUnixNano, point.TimeUnixNano, point.Attributes); err != nil {
							return err
						}
					}
					return nil
				}
				switch data := metric.Data.(type) {
				case *metricspb.Metric_Gauge:
					base.Kind = "gauge"
					err = numbers(data.Gauge.DataPoints)
				case *metricspb.Metric_Sum:
					base.Kind = "sum"
					base.Monotonic = data.Sum.IsMonotonic
					base.Temporality, err = temporality(data.Sum.AggregationTemporality)
					if err == nil {
						err = numbers(data.Sum.DataPoints)
					}
				case *metricspb.Metric_Histogram:
					base.Kind = "histogram"
					base.Temporality, err = temporality(data.Histogram.AggregationTemporality)
					if err != nil {
						return nil, err
					}
					for _, point := range data.Histogram.DataPoints {
						if point == nil {
							return nil, errors.New("nil histogram point")
						}
						if len(point.ExplicitBounds) > 0 && len(point.BucketCounts) == 0 {
							return nil, errors.New("histogram bounds require bucket counts")
						}
						if len(point.BucketCounts) > 0 {
							if len(point.BucketCounts) != len(point.ExplicitBounds)+1 {
								return nil, errors.New("histogram bucket count mismatch")
							}
							var total uint64
							for _, count := range point.BucketCounts {
								if math.MaxUint64-total < count {
									return nil, errors.New("histogram bucket overflow")
								}
								total += count
							}
							if total != point.Count {
								return nil, errors.New("histogram count differs from buckets")
							}
						}
						for i, bound := range point.ExplicitBounds {
							if math.IsNaN(bound) || math.IsInf(bound, 0) || (i > 0 && bound <= point.ExplicitBounds[i-1]) {
								return nil, errors.New("invalid histogram bounds")
							}
						}
						if point.Min != nil && point.Max != nil && *point.Min > *point.Max {
							return nil, errors.New("histogram minimum exceeds maximum")
						}
						for _, v := range []*float64{point.Sum, point.Min, point.Max} {
							if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0)) {
								return nil, errors.New("nonfinite histogram value")
							}
						}
						err = add(point, point.StartTimeUnixNano, point.TimeUnixNano, point.Attributes)
						if err != nil {
							return nil, err
						}
					}
				default:
					return nil, fmt.Errorf("unsupported metric kind for %s; supported gauge, sum, histogram", metric.Name)
				}
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return out, nil
}
