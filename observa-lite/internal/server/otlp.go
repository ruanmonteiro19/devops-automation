package server

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ruanmonteiro19/observa-lite/internal/model"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const otlpMaxBody = 4 << 20
const otlpMaxSpans = 10000

type otlpAttribute struct {
	Key   string                     `json:"key"`
	Value map[string]json.RawMessage `json:"value"`
}
type otlpLink struct {
	TraceID                string          `json:"traceId"`
	SpanID                 string          `json:"spanId"`
	TraceState             string          `json:"traceState"`
	Flags                  uint32          `json:"flags"`
	Attributes             []otlpAttribute `json:"attributes"`
	DroppedAttributesCount uint32          `json:"droppedAttributesCount"`
}
type otlpSpan struct {
	TraceState   string          `json:"traceState"`
	Flags        uint32          `json:"flags"`
	Links        []otlpLink      `json:"links"`
	TraceID      string          `json:"traceId"`
	SpanID       string          `json:"spanId"`
	ParentSpanID string          `json:"parentSpanId"`
	Name         string          `json:"name"`
	Kind         json.RawMessage `json:"kind"`
	Start        json.RawMessage `json:"startTimeUnixNano"`
	End          json.RawMessage `json:"endTimeUnixNano"`
	Attributes   []otlpAttribute `json:"attributes"`
	Events       []struct {
		Name       string          `json:"name"`
		Time       json.RawMessage `json:"timeUnixNano"`
		Attributes []otlpAttribute `json:"attributes"`
	} `json:"events"`
	Status struct {
		Code    json.RawMessage `json:"code"`
		Message string          `json:"message"`
	} `json:"status"`
}
type otlpRequest struct {
	ResourceSpans []struct {
		SchemaURL string `json:"schemaUrl"`
		Resource  struct {
			Attributes []otlpAttribute `json:"attributes"`
		} `json:"resource"`
		ScopeSpans []struct {
			SchemaURL string `json:"schemaUrl"`
			Scope     struct {
				Name       string          `json:"name"`
				Version    string          `json:"version"`
				Attributes []otlpAttribute `json:"attributes"`
			} `json:"scope"`
			Spans []otlpSpan `json:"spans"`
		} `json:"scopeSpans"`
	} `json:"resourceSpans"`
}

// ingestTraces accepts OTLP/HTTP JSON and protobuf exports. Authentication is applied by the route.
// Unsupported encodings fail explicitly; no telemetry is synthesized from discovery.
func (s *Server) ingestTraces(w http.ResponseWriter, r *http.Request) {
	typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || (typ != "application/json" && typ != "application/x-protobuf") {
		http.Error(w, "OTLP receiver supports application/json or application/x-protobuf", http.StatusUnsupportedMediaType)
		return
	}
	var reader io.Reader = http.MaxBytesReader(w, r.Body, otlpMaxBody)
	switch r.Header.Get("Content-Encoding") {
	case "", "identity":
	case "gzip":
		gz, err := gzip.NewReader(reader)
		if err != nil {
			http.Error(w, "invalid gzip body", http.StatusBadRequest)
			return
		}
		defer gz.Close()
		reader = gz
	default:
		http.Error(w, "unsupported content encoding", http.StatusUnsupportedMediaType)
		return
	}
	// Limit both compressed bytes and decompressed bytes, including gzip bombs.
	body, err := io.ReadAll(io.LimitReader(reader, otlpMaxBody+1))
	if err != nil {
		otlpDecodeError(w, err)
		return
	}
	if len(body) > otlpMaxBody {
		http.Error(w, "OTLP payload exceeds 4 MiB", http.StatusRequestEntityTooLarge)
		return
	}
	var request otlpRequest
	if typ == "application/x-protobuf" {
		var export collectortrace.ExportTraceServiceRequest
		if err = proto.Unmarshal(body, &export); err != nil {
			http.Error(w, "invalid OTLP protobuf", http.StatusBadRequest)
			return
		}
		request, err = otlpFromProto(&export)
	} else {
		if strings.TrimSpace(string(body)) == "null" {
			http.Error(w, "invalid OTLP JSON", http.StatusBadRequest)
			return
		}
		err = json.Unmarshal(body, &request)
	}
	if err != nil {
		otlpDecodeError(w, err)
		return
	}
	spans, err := convertOTLP(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(spans) > 0 {
		if err = s.Store.IngestSpans(spans); err != nil {
			http.Error(w, "trace storage failed", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", typ)
	w.WriteHeader(http.StatusOK)
	if typ == "application/json" {
		_, _ = w.Write([]byte("{}"))
	}
}
func otlpDecodeError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		http.Error(w, "OTLP payload exceeds 4 MiB", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "invalid OTLP JSON", http.StatusBadRequest)
}
func otlpID(value string, length int) bool {
	if len(value) != length {
		return false
	}
	b, err := hex.DecodeString(value)
	if err != nil {
		return false
	}
	for _, v := range b {
		if v != 0 {
			return true
		}
	}
	return false
}
func otlpNano(raw json.RawMessage) (time.Time, error) {
	value := strings.Trim(string(raw), "\"")
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}, errors.New("invalid span timestamp")
	}
	return time.Unix(0, n).UTC(), nil
}
func otlpEnum(raw json.RawMessage, names []string, prefix string) (string, error) {
	if len(raw) == 0 {
		return names[0], nil
	}
	value := strings.Trim(string(raw), "\"")
	if n, err := strconv.Atoi(value); err == nil && n >= 0 && n < len(names) {
		return names[n], nil
	}
	for _, name := range names {
		if value == prefix+strings.ToUpper(name) {
			return name, nil
		}
	}
	return "", errors.New("invalid OTLP enum")
}
func otlpAttributes(attrs []otlpAttribute) (map[string]any, error) {
	return otlpAttributeMap(attrs, 0)
}
func otlpAttributeMap(attrs []otlpAttribute, depth int) (map[string]any, error) {
	if depth > 32 {
		return nil, errors.New("OTLP attribute nesting exceeds limit")
	}
	values := make(map[string]any, len(attrs))
	for _, a := range attrs {
		if a.Key == "" {
			return nil, errors.New("invalid OTLP attribute key")
		}
		v, err := otlpValue(a.Value, depth)
		if err != nil {
			return nil, err
		}
		values[a.Key] = v
	}
	return values, nil
}
func otlpValue(value map[string]json.RawMessage, depth int) (any, error) {
	if depth > 32 || len(value) != 1 {
		return nil, errors.New("invalid OTLP attribute value")
	}
	for kind, raw := range value {
		if string(raw) == "null" {
			return nil, errors.New("null OTLP attribute value")
		}
		switch kind {
		case "stringValue", "bytesValue":
			var v string
			if json.Unmarshal(raw, &v) != nil {
				return nil, errors.New("invalid string attribute")
			}
			return v, nil
		case "intValue":
			text := strings.Trim(string(raw), "\"")
			if _, err := strconv.ParseInt(text, 10, 64); err != nil {
				return nil, errors.New("invalid integer attribute")
			}
			return text, nil
		case "boolValue":
			var v bool
			if json.Unmarshal(raw, &v) != nil {
				return nil, errors.New("invalid boolean attribute")
			}
			return v, nil
		case "doubleValue":
			var v float64
			if json.Unmarshal(raw, &v) != nil {
				return nil, errors.New("invalid double attribute")
			}
			return v, nil
		case "arrayValue":
			var array struct {
				Values []map[string]json.RawMessage `json:"values"`
			}
			if json.Unmarshal(raw, &array) != nil {
				return nil, errors.New("invalid array attribute")
			}
			result := make([]any, 0, len(array.Values))
			for _, item := range array.Values {
				v, err := otlpValue(item, depth+1)
				if err != nil {
					return nil, err
				}
				result = append(result, v)
			}
			return result, nil
		case "kvlistValue":
			var list struct {
				Values []otlpAttribute `json:"values"`
			}
			if json.Unmarshal(raw, &list) != nil {
				return nil, errors.New("invalid key-value attribute")
			}
			return otlpAttributeMap(list.Values, depth+1)
		}
	}
	return nil, errors.New("unsupported OTLP attribute value")
}
func convertOTLP(request otlpRequest) ([]model.Span, error) {
	result := []model.Span{}
	for _, resource := range request.ResourceSpans {
		attrs, err := otlpAttributes(resource.Resource.Attributes)
		if err != nil {
			return nil, err
		}
		service, _ := attrs["service.name"].(string)
		if strings.TrimSpace(service) == "" {
			service = "unknown_service"
		}
		if len(service) > 512 {
			return nil, errors.New("service name exceeds 512 bytes")
		}
		for _, scope := range resource.ScopeSpans {
			scopeAttrs, err := otlpAttributes(scope.Scope.Attributes)
			if err != nil {
				return nil, err
			}
			for _, span := range scope.Spans {
				if len(result) >= otlpMaxSpans {
					return nil, errors.New("OTLP batch exceeds 10000 spans")
				}
				if !otlpID(span.TraceID, 32) || !otlpID(span.SpanID, 16) || (span.ParentSpanID != "" && !otlpID(span.ParentSpanID, 16)) || strings.TrimSpace(span.Name) == "" || len(span.Name) > 4096 || strings.EqualFold(span.ParentSpanID, span.SpanID) {
					return nil, errors.New("invalid span identity or name")
				}
				start, err := otlpNano(span.Start)
				if err != nil {
					return nil, err
				}
				end, err := otlpNano(span.End)
				if err != nil {
					return nil, err
				}
				if start.After(time.Now().Add(5 * time.Minute)) {
					return nil, errors.New("span starts in the future")
				}
				if end.Before(start) {
					return nil, errors.New("span end precedes start")
				}
				kind, err := otlpEnum(span.Kind, []string{"unspecified", "internal", "server", "client", "producer", "consumer"}, "SPAN_KIND_")
				if err != nil {
					return nil, err
				}
				status, err := otlpEnum(span.Status.Code, []string{"unset", "ok", "error"}, "STATUS_CODE_")
				if err != nil {
					return nil, err
				}
				attributes, err := otlpAttributes(span.Attributes)
				if err != nil {
					return nil, err
				}
				events := []model.SpanEvent{}
				for _, event := range span.Events {
					at, err := otlpNano(event.Time)
					if err != nil {
						return nil, err
					}
					ea, err := otlpAttributes(event.Attributes)
					if err != nil {
						return nil, err
					}
					events = append(events, model.SpanEvent{Name: event.Name, Time: at, Attributes: ea})
				}
				namespace, _ := attrs["service.namespace"].(string)
				instance, _ := attrs["service.instance.id"].(string)
				converted := model.Span{TraceID: strings.ToLower(span.TraceID), SpanID: strings.ToLower(span.SpanID), ParentSpanID: strings.ToLower(span.ParentSpanID), Name: span.Name, ServiceName: service, ServiceNamespace: namespace, ServiceInstanceID: instance, Kind: kind, Status: status, StatusMessage: span.Status.Message, StartTime: start, EndTime: end, Attributes: attributes, ResourceAttributes: attrs, Events: events}
				converted.TraceState, converted.Flags = span.TraceState, span.Flags
				converted.Scope = model.InstrumentationScope{Name: scope.Scope.Name, Version: scope.Scope.Version, Attributes: scopeAttrs}
				converted.ScopeSchemaURL, converted.ResourceSchemaURL = scope.SchemaURL, resource.SchemaURL
				for _, link := range span.Links {
					if !otlpID(link.TraceID, 32) || !otlpID(link.SpanID, 16) {
						return nil, errors.New("invalid span link identifiers")
					}
					la, err := otlpAttributes(link.Attributes)
					if err != nil {
						return nil, err
					}
					converted.Links = append(converted.Links, model.SpanLink{TraceID: strings.ToLower(link.TraceID), SpanID: strings.ToLower(link.SpanID), TraceState: link.TraceState, Flags: link.Flags, Attributes: la, DroppedAttributesCount: link.DroppedAttributesCount})
				}
				encoded, _ := json.Marshal(converted)
				if len(encoded) > 64<<10 {
					return nil, errors.New("individual span exceeds 64 KiB")
				}
				result = append(result, converted)
			}
		}
	}
	return result, nil
}

// protojson uses base64 for bytes, whereas OTLP JSON defines hexadecimal IDs.
// Convert with the official messages, then restore those IDs from their wire bytes.
func otlpFromProto(export *collectortrace.ExportTraceServiceRequest) (otlpRequest, error) {
	var request otlpRequest
	body, err := protojson.Marshal(export)
	if err != nil {
		return request, err
	}
	if err = json.Unmarshal(body, &request); err != nil {
		return request, err
	}
	for i, resource := range export.ResourceSpans {
		for j, scope := range resource.ScopeSpans {
			for k, span := range scope.Spans {
				target := &request.ResourceSpans[i].ScopeSpans[j].Spans[k]
				target.TraceID = hex.EncodeToString(span.TraceId)
				target.SpanID = hex.EncodeToString(span.SpanId)
				target.ParentSpanID = hex.EncodeToString(span.ParentSpanId)
				for n, link := range span.Links {
					target.Links[n].TraceID = hex.EncodeToString(link.TraceId)
					target.Links[n].SpanID = hex.EncodeToString(link.SpanId)
				}
			}
		}
	}
	return request, nil
}
