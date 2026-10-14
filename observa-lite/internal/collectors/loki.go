package collectors

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "net/url"
    "strconv"
    "strings"
    "time"

    "github.com/ruanmonteiro19/observa-lite/internal/model"
)

// QueryLogs queries a configured Loki backend. Observa Lite does not persist or index log contents.
// Queries have a bounded time window, row limit, response size and timeout.
func (r Runner) QueryLogs(ctx context.Context, source model.DataSource, expr string, minutes, limit int) ([]map[string]interface{}, error) {
    if source.Kind != "loki" { return nil, errors.New("datasource must be Loki") }
    if len(expr) == 0 || len(expr) > 512 || !strings.Contains(expr, "{") || !strings.Contains(expr, "}") || strings.ContainsAny(expr, "\r\n") {
        return nil, errors.New("invalid LogQL selector (max 512 characters)")
    }
    if minutes < 1 || minutes > 1440 { return nil, errors.New("time window must be between 1 and 1440 minutes") }
    if limit < 1 || limit > 200 { return nil, errors.New("max 200 log entries per query") }
    u, err := url.Parse(source.URL)
    if err != nil || u == nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
        return nil, errors.New("invalid Loki URL")
    }
    u.Path = strings.TrimSuffix(u.Path, "/") + "/loki/api/v1/query_range"
    end := time.Now().UTC()
    q := u.Query()
    q.Set("query", expr)
    q.Set("start", strconv.FormatInt(end.Add(-time.Duration(minutes)*time.Minute).UnixNano(), 10))
    q.Set("end", strconv.FormatInt(end.UnixNano(), 10))
    q.Set("limit", strconv.Itoa(limit))
    q.Set("direction", "BACKWARD")
    u.RawQuery = q.Encode()

    body, status, _, err := r.httpGet(ctx, u.String(), source.SecretEnv, 8*time.Second)
    if err != nil { return nil, err }
    if status != 200 { return nil, fmt.Errorf("Loki upstream HTTP %d", status) }
    var payload map[string]interface{}
    if err := json.Unmarshal(body, &payload); err != nil { return nil, errors.New("invalid Loki JSON response") }
    if payload["status"] != "success" { return nil, errors.New("Loki query unsuccessful") }
    data, ok := payload["data"].(map[string]interface{})
    if !ok || data["resultType"] != "streams" { return nil, errors.New("Loki did not return log streams") }
    streams, ok := data["result"].([]interface{})
    if !ok { return nil, errors.New("invalid Loki result array") }
    rows := make([]map[string]interface{}, 0, limit)
    for _, stream := range streams {
        item, ok := stream.(map[string]interface{})
        if !ok { continue }
        labels, _ := item["stream"].(map[string]interface{})
        values, _ := item["values"].([]interface{})
        for _, entry := range values {
            pair, ok := entry.([]interface{})
            if !ok || len(pair) < 2 { continue }
            nanos, err := strconv.ParseInt(fmt.Sprint(pair[0]), 10, 64)
            if err != nil { continue }
            line, ok := pair[1].(string)
            if !ok { continue }
            if len(line) > 16384 { line = line[:16384] + " [truncado]" }
            rows = append(rows, map[string]interface{}{
                "at": time.Unix(0, nanos).UTC().Format(time.RFC3339Nano),
                "line": line,
                "labels": labels,
            })
            if len(rows) >= limit { return rows, nil }
        }
    }
    return rows, nil
}
