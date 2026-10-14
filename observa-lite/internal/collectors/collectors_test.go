package collectors

import (
	"context"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTemplateHTTP(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer s.Close()
	ok, _, msg := (Runner{AllowPrivate: true}).Check(context.Background(), model.Template{Kind: "http", Target: s.URL, TimeoutSeconds: 2})
	if !ok {
		t.Fatalf("%s", msg)
	}
}
func TestPreventPrivateByDefault(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer s.Close()
	ok, _, _ := (Runner{}).Check(context.Background(), model.Template{Kind: "http", Target: s.URL, TimeoutSeconds: 2})
	if ok {
		t.Fatal("localhost SSRF allowed")
	}
}
func TestJSONSource(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"api":{"latency":42}}`))
	}))
	defer s.Close()
	result, e := (Runner{AllowPrivate: true}).Query(context.Background(), model.DataSource{Kind: "json", URL: s.URL}, "api.latency")
	if e != nil || result.Value != 42 {
		t.Fatalf("%+v %v", result, e)
	}
}
