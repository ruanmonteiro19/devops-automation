package server

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ruanmonteiro19/observa-lite/internal/store"
	logscollector "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metricscollector "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func signalString(key, value string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}}}
}
func testLogsExport() *logscollector.ExportLogsServiceRequest {
	now := uint64(time.Now().Add(-time.Second).UnixNano())
	trace, _ := hex.DecodeString("0123456789abcdef0123456789abcdef")
	span, _ := hex.DecodeString("0123456789abcdef")
	return &logscollector.ExportLogsServiceRequest{ResourceLogs: []*logspb.ResourceLogs{{Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{signalString("service.name", "checkout"), signalString("service.namespace", "shop")}}, ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{{TimeUnixNano: now, ObservedTimeUnixNano: now, TraceId: trace, SpanId: span, SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_ERROR, SeverityText: "ERROR", Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "payment failed"}}, Attributes: []*commonpb.KeyValue{signalString("event.name", "payment")}}}}}}}}
}
func testMetricsExport() *metricscollector.ExportMetricsServiceRequest {
	now := uint64(time.Now().Add(-time.Second).UnixNano())
	sum := 4.5
	min, max := 1.0, 3.5
	return &metricscollector.ExportMetricsServiceRequest{ResourceMetrics: []*metricspb.ResourceMetrics{{Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{signalString("service.name", "checkout")}}, ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: []*metricspb.Metric{
		{Name: "queue.depth", Unit: "1", Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{DataPoints: []*metricspb.NumberDataPoint{{TimeUnixNano: now, Value: &metricspb.NumberDataPoint_AsInt{AsInt: 9007199254740993}, Attributes: []*commonpb.KeyValue{signalString("queue", "orders")}}}}}},
		{Name: "requests", Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE, IsMonotonic: true, DataPoints: []*metricspb.NumberDataPoint{{TimeUnixNano: now, StartTimeUnixNano: now - 1000000000, Value: &metricspb.NumberDataPoint_AsInt{AsInt: 10}}}}}},
		{Name: "latency", Unit: "ms", Data: &metricspb.Metric_Histogram{Histogram: &metricspb.Histogram{AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA, DataPoints: []*metricspb.HistogramDataPoint{{TimeUnixNano: now, Count: 2, Sum: &sum, Min: &min, Max: &max, ExplicitBounds: []float64{2}, BucketCounts: []uint64{1, 1}}}}}},
	}}}}}}
}
func signalWire(t *testing.T, message proto.Message, typ string) []byte {
	t.Helper()
	var body []byte
	var err error
	if typ == "application/json" {
		body, err = protojson.Marshal(message)
		// Apply the OTLP JSON special hex rule only to identity bytes.
		trace, _ := hex.DecodeString("0123456789abcdef0123456789abcdef")
		span, _ := hex.DecodeString("0123456789abcdef")
		body = bytes.ReplaceAll(body, []byte(base64.StdEncoding.EncodeToString(trace)), []byte(hex.EncodeToString(trace)))
		body = bytes.ReplaceAll(body, []byte(base64.StdEncoding.EncodeToString(span)), []byte(hex.EncodeToString(span)))
	} else {
		body, err = proto.Marshal(message)
	}
	if err != nil {
		t.Fatal(err)
	}
	return body
}
func TestOTLPSignalsJSONProtobufGzipPersistence(t *testing.T) {
	logs := testLogsExport()
	metrics := testMetricsExport()
	wantLogs, err := convertLogs(logs)
	if err != nil {
		t.Fatal(err)
	}
	wantMetrics, err := convertMetrics(metrics)
	if err != nil {
		t.Fatal(err)
	}
	if wantMetrics[0].Data["asInt"] != "9007199254740993" || !wantMetrics[1].Monotonic || wantMetrics[1].Temporality != "cumulative" || wantMetrics[2].Kind != "histogram" || wantMetrics[2].Data["sum"] != 4.5 {
		t.Fatalf("metric semantics lost: %+v", wantMetrics)
	}
	for _, typ := range []string{"application/json", "application/x-protobuf"} {
		for _, compressed := range []bool{false, true} {
			st, err := store.New(filepath.Join(t.TempDir(), "signals.json"))
			if err != nil {
				t.Fatal(err)
			}
			srv := New(st, "admin-token-12345678901234567890", "agent-token-12345678901234567890", false)
			for _, test := range []struct {
				message proto.Message
				handler http.HandlerFunc
			}{{logs, srv.ingestLogs}, {metrics, srv.ingestMetrics}} {
				body := signalWire(t, test.message, typ)
				encoding := ""
				if compressed {
					var b bytes.Buffer
					gz := gzip.NewWriter(&b)
					_, _ = gz.Write(body)
					if err = gz.Close(); err != nil {
						t.Fatal(err)
					}
					body = b.Bytes()
					encoding = "gzip"
				}
				for i := 0; i < 2; i++ {
					req := httptest.NewRequest("POST", "/v1/signals", bytes.NewReader(body))
					req.Header.Set("Content-Type", typ)
					req.Header.Set("Content-Encoding", encoding)
					req.Header.Set("X-API-Key", srv.AgentToken)
					rr := httptest.NewRecorder()
					srv.agentAuth(test.handler)(rr, req)
					if rr.Code != 200 || rr.Header().Get("Content-Type") != typ {
						t.Fatalf("export failed %s gzip=%v: %d %s", typ, compressed, rr.Code, rr.Body.String())
					}
					if typ == "application/x-protobuf" && rr.Body.Len() != 0 {
						t.Fatal("protobuf response must be empty protobuf message")
					}
				}
			}
			gotLogs := st.SignalsLogs(time.Time{}, time.Time{}, "", "", 10)
			gotMetrics := st.SignalsMetrics(time.Time{}, time.Time{}, "", "", 10)
			if !reflect.DeepEqual(gotLogs, wantLogs) {
				t.Fatalf("log parity failed: got %+v want %+v", gotLogs, wantLogs)
			}
			// Retrieval sorts newest first with deterministic same-time ordering.
			if len(gotMetrics) != 3 {
				t.Fatalf("metric export/retry count %d", len(gotMetrics))
			}
			byName := map[string]any{}
			for _, point := range gotMetrics {
				byName[point.Name] = point.Data
			}
			for _, point := range wantMetrics {
				if !reflect.DeepEqual(byName[point.Name], point.Data) {
					t.Fatalf("metric parity failed for %s", point.Name)
				}
			}
			if len(st.SignalsLogs(time.Time{}, time.Time{}, "0123456789abcdef0123456789abcdef", "checkout", 10)) != 1 || len(st.SignalsMetrics(time.Time{}, time.Time{}, "latency", "checkout", 10)) != 1 {
				t.Fatal("signal query filters failed")
			}
		}
	}
}
func TestOTLPSignalsMalformedBatchesDoNotMutate(t *testing.T) {
	logs := testLogsExport()
	bad := proto.Clone(logs.ResourceLogs[0].ScopeLogs[0].LogRecords[0]).(*logspb.LogRecord)
	bad.TraceId = []byte{1}
	logs.ResourceLogs[0].ScopeLogs[0].LogRecords = append(logs.ResourceLogs[0].ScopeLogs[0].LogRecords, bad)
	metrics := testMetricsExport()
	metrics.ResourceMetrics[0].ScopeMetrics[0].Metrics[2].GetHistogram().DataPoints[0].Count = 99
	for _, test := range []struct {
		message proto.Message
		handler http.HandlerFunc
	}{{logs, (&Server{}).ingestLogs}, {metrics, (&Server{}).ingestMetrics}} {
		body := signalWire(t, test.message, "application/x-protobuf")
		req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/x-protobuf")
		rr := httptest.NewRecorder()
		test.handler(rr, req)
		if rr.Code != 400 {
			t.Fatalf("malformed batch accepted: %d", rr.Code)
		}
	}
}
func TestOTLPSignalsAuthAndUnsupportedMetrics(t *testing.T) {
	srv := New(nil, "admin-token-12345678901234567890", "agent-token-12345678901234567890", false)
	for _, handler := range []http.HandlerFunc{srv.ingestLogs, srv.ingestMetrics} {
		for _, token := range []string{"", srv.AdminToken, srv.AgentToken} {
			req := httptest.NewRequest("POST", "/", strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()
			srv.agentAuth(handler)(rr, req)
			want := 401
			if token == srv.AgentToken {
				want = 200
			}
			if rr.Code != want {
				t.Fatalf("signal auth %d want %d", rr.Code, want)
			}
		}
	}
	export := testMetricsExport()
	export.ResourceMetrics[0].ScopeMetrics[0].Metrics[0].Data = &metricspb.Metric_Summary{Summary: &metricspb.Summary{}}
	if _, err := convertMetrics(export); err == nil {
		t.Fatal("unsupported summary silently accepted")
	}
}

func TestOTLPMetricExemplarHexIDsMatchWireFormats(t *testing.T) {
	export := testMetricsExport()
	trace, _ := hex.DecodeString("0123456789abcdef0123456789abcdef")
	span, _ := hex.DecodeString("0123456789abcdef")
	point := export.ResourceMetrics[0].ScopeMetrics[0].Metrics[0].GetGauge().DataPoints[0]
	point.Exemplars = []*metricspb.Exemplar{{TimeUnixNano: point.TimeUnixNano, TraceId: trace, SpanId: span, Value: &metricspb.Exemplar_AsInt{AsInt: 1}}}
	var results [][]any
	for _, typ := range []string{"application/json", "application/x-protobuf"} {
		req := httptest.NewRequest("POST", "/v1/metrics", bytes.NewReader(signalWire(t, export, typ)))
		req.Header.Set("Content-Type", typ)
		rr := httptest.NewRecorder()
		var decoded metricscollector.ExportMetricsServiceRequest
		if _, ok := decodeSignal(rr, req, &decoded); !ok {
			t.Fatalf("exemplar decode failed: %d %s", rr.Code, rr.Body.String())
		}
		points, err := convertMetrics(&decoded)
		if err != nil {
			t.Fatal(err)
		}
		exemplars := points[0].Data["exemplars"].([]any)
		exemplar := exemplars[0].(map[string]any)
		if exemplar["traceId"] != "0123456789abcdef0123456789abcdef" || exemplar["spanId"] != "0123456789abcdef" {
			t.Fatalf("exemplar IDs corrupted: %+v", exemplar)
		}
		results = append(results, exemplars)
	}
	if !reflect.DeepEqual(results[0], results[1]) {
		t.Fatal("exemplar JSON/protobuf differ")
	}
}
func TestOTLPLogsRejectTrailingJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/logs", strings.NewReader("{} {}"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	(&Server{}).ingestLogs(rr, req)
	if rr.Code != 400 {
		t.Fatal("trailing log JSON accepted")
	}
}

func TestOTLPLogsMissingTimestampsUseBatchReceipt(t *testing.T) {
	for _, typ := range []string{"application/json", "application/x-protobuf"} {
		t.Run(typ, func(t *testing.T) {
			export := testLogsExport()
			record := export.ResourceLogs[0].ScopeLogs[0].LogRecords[0]
			record.TimeUnixNano = 0
			record.ObservedTimeUnixNano = 0
			st, err := store.New(filepath.Join(t.TempDir(), "signals.json"))
			if err != nil {
				t.Fatal(err)
			}
			srv := New(st, "admin-token-12345678901234567890", "agent-token-12345678901234567890", false)
			req := httptest.NewRequest("POST", "/v1/logs", bytes.NewReader(signalWire(t, export, typ)))
			req.Header.Set("Content-Type", typ)
			before := time.Now().UTC()
			rr := httptest.NewRecorder()
			srv.ingestLogs(rr, req)
			after := time.Now().UTC()
			if rr.Code != 200 {
				t.Fatalf("timestamp-free logs rejected: %d %s", rr.Code, rr.Body.String())
			}
			logs := st.SignalsLogs(before.Add(-time.Second), after.Add(time.Second), "", "checkout", 10)
			if len(logs) != 1 {
				t.Fatalf("timestamp-free log not stored: %+v", logs)
			}
			log := logs[0]
			if log.Timestamp.IsZero() || log.ObservedTimestamp.IsZero() || log.Timestamp.Before(before) || log.Timestamp.After(after) || !log.Timestamp.Equal(log.ObservedTimestamp) {
				t.Fatalf("receipt timestamps invalid: %+v", log)
			}
		})
	}
}
func TestOTLPLogsPreserveSuppliedTimesAndFillObserved(t *testing.T) {
	export := testLogsExport()
	record := export.ResourceLogs[0].ScopeLogs[0].LogRecords[0]
	supplied := time.Unix(0, int64(record.TimeUnixNano)).UTC()
	record.ObservedTimeUnixNano = 0
	before := time.Now().UTC()
	logs, err := convertLogs(export)
	after := time.Now().UTC()
	if err != nil {
		t.Fatal(err)
	}
	if !logs[0].Timestamp.Equal(supplied) || logs[0].ObservedTimestamp.Before(before) || logs[0].ObservedTimestamp.After(after) {
		t.Fatalf("supplied event/receiver time not preserved: %+v", logs[0])
	}
	record.ObservedTimeUnixNano = record.TimeUnixNano
	record.TimeUnixNano = 0
	logs, err = convertLogs(export)
	if err != nil {
		t.Fatal(err)
	}
	if !logs[0].Timestamp.Equal(supplied) || !logs[0].ObservedTimestamp.Equal(supplied) {
		t.Fatalf("observed timestamp fallback changed supplied time: %+v", logs[0])
	}
	metrics := testMetricsExport()
	metrics.ResourceMetrics[0].ScopeMetrics[0].Metrics[0].GetGauge().DataPoints[0].TimeUnixNano = 0
	if _, err := convertMetrics(metrics); err == nil {
		t.Fatal("metric timestamp became optional")
	}
}
