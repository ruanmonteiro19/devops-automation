//go:build windows

package discovery

import (
 "context"
 "encoding/json"
 "fmt"
 "net"
 "os"
 "os/exec"
 "strings"
 "time"

 "github.com/ruanmonteiro19/observa-lite/internal/model"
)

// Windows collection is read-only and uses built-in PowerShell/CIM facilities.
// It does NOT automatically instrument IIS/.NET or discover HTTP services.
const windowsScript = "$ErrorActionPreference='Stop'; " +
 "$OutputEncoding=[Console]::OutputEncoding=[Text.UTF8Encoding]::new(); " +
 "$id=(Get-ItemProperty -Path 'HKLM:\\SOFTWARE\\Microsoft\\Cryptography' -Name MachineGuid -ErrorAction SilentlyContinue).MachineGuid; " +
 "$ps=@(Get-CimInstance Win32_Process | Select-Object ProcessId,Name,WorkingSetSize); " +
 "$cs=@(Get-NetTCPConnection -ErrorAction SilentlyContinue | Select-Object OwningProcess,LocalAddress,LocalPort,RemoteAddress,RemotePort,@{N='State';E={$_.State.ToString()}}); " +
 "ConvertTo-Json -InputObject @{hostid=$id;processes=$ps;connections=$cs} -Depth 5 -Compress"

type windowsSnapshot struct {
 HostID string `json:"hostid"`
 Processes []struct {
  PID int `json:"ProcessId"`
  Name string `json:"Name"`
  WorkingSet uint64 `json:"WorkingSetSize"`
 } `json:"processes"`
 Connections []struct {
  PID int `json:"OwningProcess"`
  LocalIP string `json:"LocalAddress"`
  LocalPort int `json:"LocalPort"`
  RemoteIP string `json:"RemoteAddress"`
  RemotePort int `json:"RemotePort"`
  State string `json:"State"`
 } `json:"connections"`
}

func Detect() (model.Discovery,error) {
 hostname,err:=os.Hostname()
 if err!=nil{return model.Discovery{},err}
 ctx,cancel:=context.WithTimeout(context.Background(),20*time.Second)
 defer cancel()
 output,err:=exec.CommandContext(ctx,"powershell.exe","-NoLogo","-NoProfile","-NonInteractive","-Command",windowsScript).Output()
 if err!=nil{return model.Discovery{},fmt.Errorf("Windows discovery (PowerShell/CIM): %w",err)}
 var decoded windowsSnapshot
 if err:=json.Unmarshal(output,&decoded);err!=nil{return model.Discovery{},fmt.Errorf("decode Windows discovery: %w",err)}
 hostID:="windows:"+strings.ToLower(hostname)
 if decoded.HostID!="" {hostID="windows:"+strings.ToLower(decoded.HostID)}
 d:=model.Discovery{HostID:hostID,Hostname:hostname,OS:"Windows",CapturedAt:time.Now().UTC(),Addresses:windowsIPs(),
 Processes:make([]model.Process,0,len(decoded.Processes)),Connections:make([]model.Connection,0,len(decoded.Connections))}
 for _,p:=range decoded.Processes{
  name:=strings.TrimSpace(p.Name)
  if p.PID<0 || name==""{continue}
  d.Processes=append(d.Processes,model.Process{PID:p.PID,Name:name,Service:name,Runtime:classifyWindows(name),RSSBytes:p.WorkingSet})
 }
 for _,c:=range decoded.Connections {
  if c.PID<0 {continue}
  state:=strings.ToUpper(c.State)
  if state=="LISTENING" || state=="LISTEN" {state="LISTEN"}
  if state=="ESTABLISHED" || state=="SYN_SENT" || state=="CLOSE_WAIT" || state=="LISTEN" {
   d.Connections=append(d.Connections,model.Connection{PID:c.PID,LocalIP:c.LocalIP,LocalPort:c.LocalPort,RemoteIP:c.RemoteIP,RemotePort:c.RemotePort,State:state})
  }
 }
 return d,nil
}
func classifyWindows(n string) string {
 n=strings.ToLower(strings.TrimSuffix(n,".exe"))
 switch {
 case n=="java" || n=="javaw":return "java"
 case n=="w3wp" || n=="dotnet" || n=="iisexpress":return "dotnet"
 case n=="node":return "nodejs"
 case strings.HasPrefix(n,"python"):return "python"
 case strings.HasPrefix(n,"sqlservr"):return "sqlserver"
 case n=="mysqld":return "mysql"
 default:return "process"
 }
}
func windowsIPs()[]string{
 out:=[]string{}
 ifaces,_:=net.Interfaces()
 for _,iface:=range ifaces{
  addrs,_:=iface.Addrs()
  for _,a:=range addrs{
   ip,_,err:=net.ParseCIDR(a.String())
   if err==nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast(){out=append(out,ip.String())}
  }
 }
 return out
}
