package routing
import(
 "context"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "testing"
)
func TestDiscoverAndFailover(t *testing.T){
 calls:=0
 first:=httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){calls++;w.WriteHeader(503)}))
 defer first.Close()
 second:=httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.Header.Get("X-API-Key")!="super-secret-test-token-12345"{t.Error("token missing")}
  if r.URL.Path!="/api/v1/ingest"{t.Error("wrong path")}
  w.WriteHeader(202)
 }))
 defer second.Close()
 authority:=httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.URL.Query().Get("network_zone")!="zone-a" || r.URL.Query().Get("allow_default_fallback")!="true"{
   t.Errorf("wrong policy: %v",r.URL)
  }
  if r.Header.Get("X-API-Key")!="super-secret-test-token-12345"{t.Error("control-plane token missing")}
  json.NewEncoder(w).Encode(map[string]any{"endpoints":[]string{first.URL,second.URL},"valid_for_seconds":60})
 }))
 defer authority.Close()
 // httptest uses distinct self-signed CAs; use one explicitly trusted client for
 // this local test only, not insecure TLS verification.
 roots:=first.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
 // The server can use an httptest trusted TLS test client for transport failover by
 // adding both gateway CAs to an explicit pool.
 _=roots
 cli:=authority.Client()
 // Each new TLSServer has a separate self-signed certificate; inject certificates from fixtures
 // by constructing a shared RootCAs pool for all three.
 pool:=cli.Transport.(*http.Transport).TLSClientConfig.RootCAs
 pool.AddCert(first.Certificate())
 pool.AddCert(second.Certificate())
 cli.Transport.(*http.Transport).TLSClientConfig.RootCAs=pool
 cli.CheckRedirect=func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}
 endpoints,err:=Discover(context.Background(),cli,authority.URL,"super-secret-test-token-12345","zone-a",true)
 if err!=nil{t.Fatal(err)}
 if len(endpoints)!=2{t.Fatalf("got %d endpoints",len(endpoints))}
 selected,err:=Deliver(context.Background(),cli,endpoints,"super-secret-test-token-12345",[]byte("{}"))
 if err!=nil{t.Fatal(err)}
 if selected!=second.URL||calls!=1{t.Fatalf("selected=%s, first calls=%d",selected,calls)}
}
func TestRejectUnsafeOrigins(t *testing.T){
 for _,raw:=range []string{"http://malicious.test:80","https://user:pass@example.com","https://valid.test/ingest","https://valid.test?token=secret","file:///tmp/a"}{
  if TrustedOrigin(raw){t.Errorf("accepted unsafe origin: %s",raw)}
 }
}
