package server

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ruanmonteiro19/observa-lite/internal/store"
)

const otlpValidSpan = `{"traceId":"0123456789abcdef0123456789abcdef","spanId":"0123456789abcdef","name":"GET /checkout","kind":"SPAN_KIND_SERVER","startTimeUnixNano":"1700000000000000000","endTimeUnixNano":"1700000000050000000","attributes":[{"key":"http.response.status_code","value":{"intValue":"503"}}],"status":{"code":2,"message":"unavailable"},"events":[{"name":"exception","timeUnixNano":"1700000000010000000","attributes":[{"key":"exception.message","value":{"stringValue":"database unavailable"}}]}]}`

func otlpPayload(spans string) string {
	return `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"checkout"}},{"key":"service.namespace","value":{"stringValue":"shop"}}]},"scopeSpans":[{"scope":{"name":"test"},"spans":[` + spans + `]}]}]}`
}
func TestOTLPConversion(t *testing.T) {
	var payload otlpRequest
	if err := json.Unmarshal([]byte(otlpPayload(otlpValidSpan)), &payload); err != nil {
		t.Fatal(err)
	}
	spans, err := convertOTLP(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 {
		t.Fatalf("span count %d", len(spans))
	}
	span := spans[0]
	if span.ServiceName != "checkout" || span.ServiceNamespace != "shop" || span.Kind != "server" || span.Status != "error" || span.EndTime.Sub(span.StartTime).Milliseconds() != 50 || span.Attributes["http.response.status_code"] != "503" || len(span.Events) != 1 || span.Events[0].Name != "exception" {
		t.Fatalf("conversion lost OTLP fields: %+v", span)
	}
}
func TestOTLPRejectsInvalidBatchBeforeStorage(t *testing.T) {
	// A nil store makes any attempt to mutate on a rejected batch fail the test.
	s := &Server{}
	invalid := strings.Replace(otlpValidSpan, `"spanId":"0123456789abcdef"`, `"spanId":"0000000000000000"`, 1)
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", strings.NewReader(otlpPayload(otlpValidSpan+","+invalid)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ingestTraces(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
}
func TestOTLPHandlerFormatsAndLimits(t *testing.T) {
	cases := []struct {
		name, typ, encoding, body string
		status                    int
	}{
		{"bad protobuf", "application/x-protobuf", "", "x", 400},
		{"bad gzip", "application/json", "gzip", "{}", 400},
		{"trailing JSON", "application/json", "", "{} {}", 400},
		{"bad JSON", "application/json", "", "{", 400},
		{"too large", "application/json", "", `{"resourceSpans":[],"padding":"` + strings.Repeat("x", otlpMaxBody) + `"}`, 413},
		{"empty export", "application/json", "", "{}", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/traces", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.typ)
			req.Header.Set("Content-Encoding", tc.encoding)
			rr := httptest.NewRecorder()
			(&Server{}).ingestTraces(rr, req)
			if rr.Code != tc.status {
				t.Fatalf("status %d want %d: %s", rr.Code, tc.status, rr.Body.String())
			}
		})
	}
}
func TestOTLPHTTPPersistsValidExport(t *testing.T) {
	file := filepath.Join(t.TempDir(), "telemetry.json")
	st, err := store.New(file)
	if err != nil {
		t.Fatal(err)
	}
	s := New(st, "admin", "agent", false)
	now := time.Now().Add(-time.Second).UnixNano()
	span := strings.NewReplacer("1700000000000000000", strconv.FormatInt(now, 10), "1700000000050000000", strconv.FormatInt(now+50000000, 10), "1700000000010000000", strconv.FormatInt(now+10000000, 10)).Replace(otlpValidSpan)
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", strings.NewReader(otlpPayload(span)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ingestTraces(rr, req)
	if rr.Code != 200 || rr.Body.String() != "{}" {
		t.Fatalf("OTLP export failed: %d %s", rr.Code, rr.Body.String())
	}
	reloaded, err := store.New(file)
	if err != nil {
		t.Fatal(err)
	}
	trace, ok := reloaded.APMTrace("0123456789abcdef0123456789abcdef")
	if !ok || len(trace.Spans) != 1 || trace.Spans[0].ServiceName != "checkout" {
		t.Fatalf("export was not persisted: %+v", trace)
	}
}
func TestOTLPRejectsReversedTimesAndInvalidEnums(t *testing.T) {
	for _, bad := range []string{
		strings.Replace(otlpValidSpan, "1700000000050000000", "1600000000000000000", 1),
		strings.Replace(otlpValidSpan, "SPAN_KIND_SERVER", "SPAN_KIND_INVALID", 1),
		strings.Replace(otlpValidSpan, `"code":2`, `"code":99`, 1),
	} {
		var payload otlpRequest
		if err := json.Unmarshal([]byte(otlpPayload(bad)), &payload); err != nil {
			t.Fatal(err)
		}
		if _, err := convertOTLP(payload); err == nil {
			t.Fatal("invalid batch accepted")
		}
	}
}

func TestOTLPProtobufMatchesJSONAndGzip(t *testing.T) {
	traceID, _ := hex.DecodeString("0123456789abcdef0123456789abcdef")
	spanID, _ := hex.DecodeString("0123456789abcdef")
	str := func(key, value string) *commonpb.KeyValue {
		return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}}}
	}
	export := &collectortrace.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{str("service.name", "checkout"), str("service.namespace", "shop")}},
		ScopeSpans: []*tracepb.ScopeSpans{{Scope: &commonpb.InstrumentationScope{Name: "test"}, Spans: []*tracepb.Span{{TraceId: traceID, SpanId: spanID, Name: "GET /checkout", Kind: tracepb.Span_SPAN_KIND_SERVER, StartTimeUnixNano: 1700000000000000000, EndTimeUnixNano: 1700000000050000000,
			Attributes: []*commonpb.KeyValue{{Key: "http.response.status_code", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 503}}}},
			Status:     &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: "unavailable"},
			Events:     []*tracepb.Span_Event{{Name: "exception", TimeUnixNano: 1700000000010000000, Attributes: []*commonpb.KeyValue{str("exception.message", "database unavailable")}}},
		}}}},
	}}}
	converted, err := otlpFromProto(export)
	if err != nil {
		t.Fatal(err)
	}
	var jsonRequest otlpRequest
	if err = json.Unmarshal([]byte(otlpPayload(otlpValidSpan)), &jsonRequest); err != nil {
		t.Fatal(err)
	}
	want, err := convertOTLP(jsonRequest)
	if err != nil {
		t.Fatal(err)
	}
	got, err := convertOTLP(converted)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("protobuf conversion differs: got %+v want %+v", got, want)
	}
	now := time.Now().Add(-time.Second).UnixNano()
	wireSpan := export.ResourceSpans[0].ScopeSpans[0].Spans[0]
	wireSpan.StartTimeUnixNano = uint64(now)
	wireSpan.EndTimeUnixNano = uint64(now + 50000000)
	wireSpan.Events[0].TimeUnixNano = uint64(now + 10000000)
	wire, err := proto.Marshal(export)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.New(filepath.Join(t.TempDir(), "telemetry.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(st, "admin", "agent", false)
	for _, compressed := range []bool{false, true} {
		body := wire
		encoding := ""
		if compressed {
			var compressedBody bytes.Buffer
			gz := gzip.NewWriter(&compressedBody)
			if _, err = gz.Write(wire); err != nil {
				t.Fatal(err)
			}
			if err = gz.Close(); err != nil {
				t.Fatal(err)
			}
			body = compressedBody.Bytes()
			encoding = "gzip"
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/x-protobuf")
		req.Header.Set("Content-Encoding", encoding)
		rr := httptest.NewRecorder()
		s.ingestTraces(rr, req)
		if rr.Code != 200 || rr.Body.Len() != 0 || rr.Header().Get("Content-Type") != "application/x-protobuf" {
			t.Fatalf("protobuf HTTP failed: %d %s", rr.Code, rr.Body.String())
		}
	}
	trace, ok := st.APMTrace("0123456789abcdef0123456789abcdef")
	if !ok || len(trace.Spans) != 1 {
		t.Fatalf("protobuf exports did not persist and deduplicate: %+v", trace)
	}
}

func TestOTLPGzipExpandedLimit(t *testing.T) {
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, err := gz.Write([]byte(`{"padding":"` + strings.Repeat("a", otlpMaxBody) + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err = gz.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", &compressed)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	rr := httptest.NewRecorder()
	(&Server{}).ingestTraces(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("gzip expansion accepted: %d", rr.Code)
	}
}
func TestOTLPBatchCountLimit(t *testing.T) {
	var payload otlpRequest
	if err := json.Unmarshal([]byte(otlpPayload(otlpValidSpan)), &payload); err != nil {
		t.Fatal(err)
	}
	span := payload.ResourceSpans[0].ScopeSpans[0].Spans[0]
	payload.ResourceSpans[0].ScopeSpans[0].Spans = make([]otlpSpan, otlpMaxSpans+1)
	for i := range payload.ResourceSpans[0].ScopeSpans[0].Spans {
		payload.ResourceSpans[0].ScopeSpans[0].Spans[i] = span
	}
	if _, err := convertOTLP(payload); err == nil {
		t.Fatal("oversized span count accepted")
	}
}
func TestOTLPNestedAttributesPreserveTypes(t *testing.T) {
	var attrs []otlpAttribute
	if err := json.Unmarshal([]byte(`[{"key":"array","value":{"arrayValue":{"values":[{"boolValue":true},{"doubleValue":1.5}]}}},{"key":"map","value":{"kvlistValue":{"values":[{"key":"name","value":{"stringValue":"db"}}]}}}]`), &attrs); err != nil {
		t.Fatal(err)
	}
	got, err := otlpAttributes(attrs)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"array": []any{true, 1.5}, "map": map[string]any{"name": "db"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("attribute types changed: %#v", got)
	}
}

func TestOTLPHTTPInputErrorsAreNotPersistenceErrors(t *testing.T) {
	future := strconv.FormatInt(time.Now().Add(10*time.Minute).UnixNano(), 10)
	futureEnd := strconv.FormatInt(time.Now().Add(11*time.Minute).UnixNano(), 10)
	futureSpan := strings.NewReplacer("1700000000000000000", future, "1700000000050000000", futureEnd).Replace(otlpValidSpan)
	largeSpan := strings.Replace(otlpValidSpan, "database unavailable", strings.Repeat("x", 65<<10), 1)
	for name, span := range map[string]string{"future timestamp": futureSpan, "oversized span": largeSpan} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/traces", strings.NewReader(otlpPayload(span)))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			(&Server{}).ingestTraces(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("input error returned %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
	blocked := filepath.Join(t.TempDir(), "blocked")
	st, err := store.New(filepath.Join(blocked, "telemetry.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", strings.NewReader(otlpPayload(otlpValidSpan)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	(&Server{Store: st}).ingestTraces(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("persistence error returned %d: %s", rr.Code, rr.Body.String())
	}
}
func TestOTLPHTTPRouteRequiresAgentCredential(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "telemetry.json"))
	if err != nil {
		t.Fatal(err)
	}
	srv := New(st, "admin-token-12345678901234567890", "agent-token-12345678901234567890", false)
	handler := srv.Handler()
	for _, token := range []string{"", srv.AdminToken, "invalid", srv.AgentToken} {
		req := httptest.NewRequest(http.MethodPost, "/v1/traces", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		want := http.StatusUnauthorized
		if token == srv.AgentToken {
			want = http.StatusOK
		}
		if rr.Code != want {
			t.Fatalf("credential role wrong: got %d want %d", rr.Code, want)
		}
	}
}

// Exercise both transports: protobuf bytes must become hexadecimal link IDs,
// and links to absent traces remain valid evidence rather than fabricated parents.
func TestOTLPSpanLinksAndInstrumentation(t *testing.T) {
	source := &collectortrace.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		SchemaUrl: "resource-schema", ScopeSpans: []*tracepb.ScopeSpans{{SchemaUrl: "scope-schema",
			Scope: &commonpb.InstrumentationScope{Name: "queue-sdk", Version: "2.0", Attributes: []*commonpb.KeyValue{{Key: "component", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "consumer"}}}}},
			Spans: []*tracepb.Span{{TraceId: bytes.Repeat([]byte{0xab}, 16), SpanId: bytes.Repeat([]byte{0xcd}, 8), Name: "consume", Kind: tracepb.Span_SPAN_KIND_CONSUMER, Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK}, StartTimeUnixNano: 1700000000000000000, EndTimeUnixNano: 1700000000001000000, Flags: 769, TraceState: "vendor=value", Links: []*tracepb.Span_Link{{TraceId: bytes.Repeat([]byte{0xef}, 16), SpanId: bytes.Repeat([]byte{0x12}, 8), Flags: 513, TraceState: "queue=value", DroppedAttributesCount: 2, Attributes: []*commonpb.KeyValue{{Key: "batch", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 7}}}}}}}},
		}},
	}}}
	request, err := otlpFromProto(source)
	if err != nil {
		t.Fatal(err)
	}
	// An OTLP JSON request contains hex IDs; serializing this intermediate
	// exercises the same decoder used by the JSON HTTP receiver.
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded otlpRequest
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, input := range []otlpRequest{request, decoded} {
		spans, err := convertOTLP(input)
		if err != nil {
			t.Fatal(err)
		}
		sp := spans[0]
		if sp.Flags != 769 || sp.TraceState != "vendor=value" || sp.Scope.Name != "queue-sdk" || sp.Scope.Version != "2.0" || sp.Scope.Attributes["component"] != "consumer" || sp.ScopeSchemaURL != "scope-schema" || sp.ResourceSchemaURL != "resource-schema" {
			t.Fatalf("lost context: %+v", sp)
		}
		if len(sp.Links) != 1 || sp.Links[0].TraceID != strings.Repeat("ef", 16) || sp.Links[0].SpanID != strings.Repeat("12", 8) || sp.Links[0].Flags != 513 || sp.Links[0].DroppedAttributesCount != 2 || sp.Links[0].Attributes["batch"] != "7" || sp.ParentSpanID != "" {
			t.Fatalf("lost or reparented link: %+v", sp)
		}
	}
	request.ResourceSpans[0].ScopeSpans[0].Spans[0].Links[0].TraceID = strings.Repeat("0", 32)
	if _, err := convertOTLP(request); err == nil {
		t.Fatal("accepted zero link ID")
	}
}
