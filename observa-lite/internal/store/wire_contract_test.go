package store

import (
  "encoding/json"
  "path/filepath"
  "testing"
  "github.com/ruanmonteiro19/observa-lite/internal/model"
)

// Regression: on a quiet machine, no observed sockets must produce [] (not null).
// This must remain true after reloading persisted state.
func TestEmptyProcessAndConnectionArrays(t *testing.T) {
 p:=filepath.Join(t.TempDir(),"state.json")
 st,err:=New(p);if err!=nil {t.Fatal(err)}
 if err:=st.Ingest(model.Discovery{HostID:"quiet-linux",Hostname:"quiet-linux",OS:"Linux"});err!=nil {t.Fatal(err)}
 for i:=0;i<2;i++ {
  current,err:=New(p);if err!=nil{t.Fatal(err)}
  b,err:=json.Marshal(current.Snapshot());if err!=nil{t.Fatal(err)}
  var d struct{Hosts []struct {
    Processes json.RawMessage `json:"processes"`
    Connections json.RawMessage `json:"connections"`
    Addresses json.RawMessage `json:"addresses"`
  } `json:"hosts"`}
  if err:=json.Unmarshal(b,&d);err!=nil{t.Fatal(err)}
  if len(d.Hosts)!=1{t.Fatalf("wanted one host, got %d",len(d.Hosts))}
  for k,v:=range map[string]json.RawMessage{"processes":d.Hosts[0].Processes,"connections":d.Hosts[0].Connections,"addresses":d.Hosts[0].Addresses} {
   if string(v)!="[]"{t.Errorf("%s expected [] got %s",k,string(v))}
  }
 }
}
