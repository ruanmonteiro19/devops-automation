package server

import (
    "errors"
    "net/http"
    "strconv"
    "strings"
)

// logs is read-only. Logs remain exclusively in the configured external Loki store.
func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
    q:=r.URL.Query()
    sourceID:=strings.TrimSpace(q.Get("source"))
    expr:=strings.TrimSpace(q.Get("query"))
    window:=60
    limit:=100
    if x:=q.Get("minutes");x!="" {
        n,e:=strconv.Atoi(x);if e!=nil || n<1 || n>1440 {fail(w,400,errors.New("minutes must be 1..1440"));return};window=n
    }
    if x:=q.Get("limit");x!="" {
        n,e:=strconv.Atoi(x);if e!=nil || n<1 || n>200 {fail(w,400,errors.New("limit must be 1..200"));return};limit=n
    }
    ds,ok:=s.Store.Source(sourceID)
    if !ok {fail(w,404,errors.New("datasource not found"));return}
    if ds.Kind!="loki" {fail(w,400,errors.New("select a Loki datasource"));return}
    if len(expr)>512 || expr=="" {fail(w,400,errors.New("LogQL query required, maximum 512 characters"));return}
    lines,e:=s.Runner.QueryLogs(r.Context(),ds,expr,window,limit)
    if e!=nil {fail(w,502,e);return}
    write(w,200,map[string]interface{}{"logs":lines,"source":ds.ID,"minutes":window,"limit":limit,"storage":"loki","persisted_by_observa":false})
}
