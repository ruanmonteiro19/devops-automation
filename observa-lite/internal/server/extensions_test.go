package server
import ("net/http";"net/http/httptest";"path/filepath";"testing";"github.com/ruanmonteiro19/observa-lite/internal/store")
func TestCapabilitiesAndExtensionsRequireAuthentication(t *testing.T) {
 st,err:=store.New(filepath.Join(t.TempDir(),"state.json"));if err!=nil {t.Fatal(err)}
 s:=New(st,"this-admin-token-is-at-least-24-chars","this-agent-token-is-at-least-24-chars",false)
 for _,path:=range []string{"/api/v1/capabilities","/api/v1/extensions"} {
  r:=httptest.NewRequest(http.MethodGet,path,nil)
  w:=httptest.NewRecorder();s.Handler().ServeHTTP(w,r)
  if w.Code!=http.StatusUnauthorized {t.Fatalf("%s without token: %d",path,w.Code)}
  r.Header.Set("X-API-Key",s.AdminToken)
  w=httptest.NewRecorder();s.Handler().ServeHTTP(w,r)
  if w.Code!=http.StatusOK {t.Fatalf("%s with token: %d: %s",path,w.Code,w.Body.String())}
 }
}
