package main
import("bytes";"net/http";"net/http/httptest";"net/url";"testing";"context")
func TestRelayAuthenticatedAndRestricted(t *testing.T) {
 upstream:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.URL.Path!="/api/v1/ingest"||r.Header.Get("X-API-Key")!="012345678901234567890123456789" {t.Errorf("bad forwarded data")}
  w.WriteHeader(http.StatusAccepted)
 }))
 defer upstream.Close()
 u,_:=url.Parse(upstream.URL)
 r:=&relay{upstream:u,httpClient:upstream.Client(),token:"012345678901234567890123456789",slots:make(chan struct{},2)}
 for _,tc:=range []struct{method,path,token string;status int}{
  {"GET","/healthz","",200},
  {"POST","/api/v1/ingest","bad",401},
  {"POST","/api/v1/anything","012345678901234567890123456789",404},
  {"POST","/api/v1/ingest","012345678901234567890123456789",202},
 }{
  q:=httptest.NewRequest(tc.method,tc.path,bytes.NewBufferString("{}")).WithContext(context.Background())
  q.Header.Set("X-API-Key",tc.token);w:=httptest.NewRecorder();r.ServeHTTP(w,q)
  if w.Code!=tc.status{t.Errorf("%s %s: got %d want %d",tc.method,tc.path,w.Code,tc.status)}
 }
}
