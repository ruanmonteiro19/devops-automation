package server

import (
 "bytes"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "path/filepath"
 "testing"
 "github.com/ruanmonteiro19/observa-lite/internal/model"
 "github.com/ruanmonteiro19/observa-lite/internal/store"
)

func TestGatewayAdminRegistrationAndAgentRouting(t *testing.T){
 db,err:=store.New(filepath.Join(t.TempDir(),"state.json"));if err!=nil{t.Fatal(err)}
 admin:="admin-token-for-testing-0123456789"
 agent:="agent-token-for-testing-0123456789"
 handler:=New(db,admin,agent,false).Handler()
 g:=model.GatewayEndpoint{ID:"gate-a",NetworkZone:"zone-a",URL:"https://gate-a.example:18443",Priority:10,Enabled:true}
 payload,_:=json.Marshal(g)
 for _,tc:=range []struct{token string;status int}{{"",401},{agent,401},{admin,200}}{
  r:=httptest.NewRequest("POST","/api/v1/gateways",bytes.NewReader(payload));r.Header.Set("X-API-Key",tc.token)
  w:=httptest.NewRecorder();handler.ServeHTTP(w,r)
  if w.Code!=tc.status{t.Fatalf("register status got %d want %d",w.Code,tc.status)}
 }
 for _,tc:=range []struct{url,token string;status int;count int}{
  {"/api/v1/agent/connectioninfo?network_zone=zone-a",agent,200,1},
  {"/api/v1/agent/connectioninfo?network_zone=zone-a",admin,401,0},
  {"/api/v1/agent/connectioninfo?network_zone=unknown",agent,503,0},
  {"/api/v1/gateways",agent,401,0},
 }{
  r:=httptest.NewRequest(http.MethodGet,tc.url,nil);r.Header.Set("X-API-Key",tc.token)
  w:=httptest.NewRecorder();handler.ServeHTTP(w,r)
  if w.Code!=tc.status{t.Errorf("%s returned %d want %d: %s",tc.url,w.Code,tc.status,w.Body.String());continue}
  if tc.count>0{
   var info model.ConnectionInfo
   if err:=json.Unmarshal(w.Body.Bytes(),&info);err!=nil{t.Fatal(err)}
   if len(info.Endpoints)!=tc.count||info.Endpoints[0]!=g.URL{t.Fatalf("connectioninfo %+v",info)}
  }
 }
}
