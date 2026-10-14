package store
import("path/filepath";"testing";"time";"github.com/ruanmonteiro19/observa-lite/internal/model")
func TestSnapshotDoesNotTreatProcessAsAPMService(t *testing.T){
 s,err:=New(filepath.Join(t.TempDir(),"state.json"));if err!=nil{t.Fatal(err)}
 d:=model.Discovery{HostID:"qa",Hostname:"qa",OS:"Linux",CapturedAt:time.Now(),Processes:[]model.Process{{PID:421,Name:"java",Service:"webapp",Runtime:"java"}},Connections:[]model.Connection{{PID:421,LocalIP:"127.0.0.1",LocalPort:8080,State:"LISTEN"}}}
 if err:=s.Ingest(d);err!=nil{t.Fatal(err)}
 a:=s.Snapshot()
 if len(a.ServiceMap.Nodes)!=0{t.Fatalf("PID is not real APM service: %+v",a.ServiceMap.Nodes)}
 if len(a.ProcessMap.Nodes)!=1||a.ProcessMap.Nodes[0].Kind!="process-group"{t.Fatalf("process map %+v",a.ProcessMap.Nodes)}
 d.Processes=nil;d.Connections=nil
 if err:=s.Ingest(d);err!=nil{t.Fatal(err)}
 b:=s.Snapshot()
 if len(b.Events)!=0{t.Fatalf("process exit is not an APM incident: %+v",b.Events)}
}
