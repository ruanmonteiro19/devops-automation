package main

import (
 "bytes"
 "net/http"
 "net/http/httptest"
 "testing"
)

func TestAgentNeverRedirectsAgentToken(t *testing.T) {
 received:=false
 destination:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.Header.Get("X-API-Key")!="" {received=true}
  w.WriteHeader(http.StatusOK)
 }))
 defer destination.Close()
 origin:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  http.Redirect(w,r,destination.URL,http.StatusTemporaryRedirect)
 }))
 defer origin.Close()
 req,err:=http.NewRequest(http.MethodPost,origin.URL,bytes.NewBufferString("{}"))
 if err!=nil {t.Fatal(err)}
 req.Header.Set("X-API-Key","secret-must-not-leak")
 tr:=http.DefaultTransport.(*http.Transport).Clone()
 client:=newAgentHTTPClient(tr)
 response,err:=client.Do(req)
 if response!=nil {response.Body.Close()}
 if err==nil {t.Fatal("agent followed or silently accepted the redirect")}
 if received {t.Fatal("agent token was forwarded to a redirect destination")}
}
