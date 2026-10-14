package routing

import(
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/url"
 "strings"
)

// TrustedOrigin rejects credential-containing, redirected and plaintext remote endpoints.
func TrustedOrigin(raw string) bool {
 u,err:=url.Parse(raw)
 if err!=nil||u.Hostname()==""||u.User!=nil||u.RawQuery!=""||u.Fragment!="" || (u.Path!=""&&u.Path!="/"){return false}
 if u.Scheme=="https"{return true}
 if u.Scheme!="http"{return false}
 ip:=net.ParseIP(u.Hostname())
 return strings.EqualFold(u.Hostname(),"localhost") || (ip!=nil&&ip.IsLoopback())
}
type connectionInfo struct{
 Endpoints []string `json:"endpoints"`
 ValidForSeconds int `json:"valid_for_seconds"`
}

// Discover accepts destinations only from a TLS-verified, authenticated central authority.
// The API does not rely on a gateway's self-reported status or DNS discovery.
func Discover(ctx context.Context, client *http.Client, authority,token,zone string,allowDefault bool)([]string,error){
 if !TrustedOrigin(authority){return nil,errors.New("unsafe connectioninfo authority")}
 origin,_:=url.Parse(authority)
 origin.Path="/api/v1/agent/connectioninfo"
 q:=url.Values{}
 q.Set("network_zone",zone)
 if allowDefault {q.Set("allow_default_fallback","true")}
 origin.RawQuery=q.Encode()
 req,err:=http.NewRequestWithContext(ctx,http.MethodGet,origin.String(),nil);if err!=nil{return nil,err}
 req.Header.Set("X-API-Key",token)
 resp,err:=client.Do(req);if err!=nil{return nil,err}
 defer resp.Body.Close()
 if resp.StatusCode!=http.StatusOK{return nil,fmt.Errorf("connectioninfo HTTP %d",resp.StatusCode)}
 var info connectionInfo
 dec:=json.NewDecoder(io.LimitReader(resp.Body,65536))
 if err:=dec.Decode(&info);err!=nil{return nil,err}
 if len(info.Endpoints)==0||len(info.Endpoints)>16 {return nil,fmt.Errorf("connectioninfo has %d endpoints (expected 1..16)",len(info.Endpoints))}
 seen:=map[string]bool{}
 result:=make([]string,0,len(info.Endpoints))
 for _,s:=range info.Endpoints{
  if !TrustedOrigin(s) || !strings.HasPrefix(s,"https://") {return nil,fmt.Errorf("invalid gateway origin")}
  if seen[s]{continue}
  seen[s]=true;result=append(result,s)
 }
 return result,nil
}

// Deliver tries each approved endpoint in order. Validation/auth failures stop
// immediately; transient network/server failures can fail over to the next relay.
// No redirect is allowed: the caller's HTTP client MUST reject redirect requests.
func Deliver(ctx context.Context,client *http.Client,endpoints []string,token string,payload []byte)(string,error){
 var lastErr error
 for _,origin:=range endpoints{
  if !TrustedOrigin(origin){return "",errors.New("unsafe ingestion origin")}
  base,_:=url.Parse(origin)
  base.Path="/api/v1/ingest"
  req,err:=http.NewRequestWithContext(ctx,http.MethodPost,base.String(),bytes.NewReader(payload))
  if err!=nil{return "",err}
  req.Header.Set("X-API-Key",token)
  req.Header.Set("Content-Type","application/json")
  resp,err:=client.Do(req)
  if err!=nil{lastErr=err;continue}
  status:=resp.StatusCode
  _,_=io.Copy(io.Discard,io.LimitReader(resp.Body,1024));_ =resp.Body.Close()
  if status==http.StatusAccepted{return origin,nil}
  if status>=500 || status==http.StatusTooManyRequests {lastErr=fmt.Errorf("gateway %s HTTP %d",base.Host,status);continue}
  return "",fmt.Errorf("gateway rejected request: HTTP %d",status)
 }
 if lastErr==nil{lastErr=errors.New("no endpoints available")}
 return "",lastErr
}
