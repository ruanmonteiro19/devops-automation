package collectors

import (
    "context"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "github.com/ruanmonteiro19/observa-lite/internal/model"
)

func TestQueryLogsBounded(t *testing.T) {
    mock:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
        if r.URL.Path!="/loki/api/v1/query_range" {t.Errorf("path: %s",r.URL.Path)}
        if got:=r.URL.Query().Get("limit"); got!="2" {t.Errorf("limit: %s",got)}
        if got:=r.URL.Query().Get("direction"); got!="BACKWARD" {t.Errorf("direction: %s",got)}
        w.Header().Set("Content-Type","application/json")
        fmt.Fprint(w,`{"status":"success","data":{"resultType":"streams","result":[{"stream":{"service":"api"},"values":[["1791980000000000000","hello"],["1791980000000000001","second"],["1791980000000000002","third"]]}]}}`)
    }))
    defer mock.Close()
    runner:=Runner{AllowPrivate:true}
    rows,err:=runner.QueryLogs(context.Background(),model.DataSource{Kind:"loki",URL:mock.URL},"{service=\"api\"}",60,2)
    if err!=nil || len(rows)!=2 {t.Fatalf("got %d rows: %v",len(rows),err)}
    if rows[0]["line"]!="hello" {t.Fatalf("wrong row: %v",rows[0])}
    for _,limit:=range []int{0,201} {_,err=runner.QueryLogs(context.Background(),model.DataSource{Kind:"loki",URL:mock.URL},"{service=\"api\"}",60,limit);if err==nil {t.Errorf("accepted limit %d",limit)}}
    _,err=runner.QueryLogs(context.Background(),model.DataSource{Kind:"loki",URL:mock.URL}, "{service=\"api\"}",1441,5)
    if err==nil {t.Fatal("allowed unbounded window")}
    _,err=runner.QueryLogs(context.Background(),model.DataSource{Kind:"loki",URL:mock.URL}, strings.Repeat("x",513),60,10)
    if err==nil {t.Fatal("allowed oversized LogQL")}
}
func TestQueryLogsRequiresLoki(t *testing.T) {
   _,e:=(Runner{AllowPrivate:true}).QueryLogs(context.Background(),model.DataSource{Kind:"prometheus",URL:"http://127.0.0.1:80"},"{job=\"x\"}",60,5)
   if e==nil {t.Fatal("must reject non-Loki sources")}
}
func TestQueryLogsPrivateBlocked(t *testing.T) {
    mock:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) { w.WriteHeader(200) }));defer mock.Close()
    _,err:=(Runner{}).QueryLogs(context.Background(),model.DataSource{Kind:"loki",URL:mock.URL},"{job="api"}",5,5)
    if err==nil {t.Fatal("expected loopback SSRF protection")}
}
