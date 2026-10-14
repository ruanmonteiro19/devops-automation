package store
import(
 "path/filepath"
 "testing"
 "github.com/ruanmonteiro19/observa-lite/internal/model"
)
func TestGatewayZonePriorityAndPersistence(t *testing.T){
 p:=filepath.Join(t.TempDir(),"state.json")
 s,e:=New(p);if e!=nil{t.Fatal(e)}
 for _,g:=range []model.GatewayEndpoint{
  {ID:"a-high",NetworkZone:"zone-a",URL:"https://a.test:443",Priority:20,Enabled:true},
  {ID:"a-low",NetworkZone:"zone-a",URL:"https://b.test:443",Priority:2,Enabled:true},
  {ID:"default",NetworkZone:"default",URL:"https://fallback.test:443",Priority:5,Enabled:true},
  {ID:"disabled",NetworkZone:"zone-a",URL:"https://offline.test:443",Priority:1,Enabled:false},
 }{if err:=s.UpsertGateway(g);err!=nil{t.Fatal(err)}}
 x:=s.ConnectionInfo("zone-a",false)
 if len(x.Endpoints)!=2 || x.Endpoints[0]!="https://b.test:443"||x.Endpoints[1]!="https://a.test:443" || x.DefaultZoneFallback{t.Fatalf("strict: %+v",x)}
 y:=s.ConnectionInfo("zone-a",true)
 if len(y.Endpoints)!=3 || y.Endpoints[2]!="https://fallback.test:443" || !y.DefaultZoneFallback{t.Fatalf("fallback: %+v",y)}
 z:=s.ConnectionInfo("zone-unknown",false)
 if len(z.Endpoints)!=0{t.Fatalf("unknown zone must be empty: %+v",z)}
 for _,invalid:=range []string{"http://insecure.test","https://hostname.test/foo","https://user:pass@hostname.test","https://hostname.test/?next=evil"}{
  if err:=s.UpsertGateway(model.GatewayEndpoint{ID:"bad",NetworkZone:"default",URL:invalid,Enabled:true});err==nil{t.Errorf("invalid URL accepted: %s",invalid)}
 }
 s2,e:=New(p);if e!=nil{t.Fatal(e)}
 if len(s2.ConnectionInfo("zone-a",false).Endpoints)!=2 {t.Fatal("gateways not persisted")}
}
