package discovery

import "testing"

func TestDecodeAddr(t *testing.T) {
	ip, port, ok := decodeAddr("0100007F:1F90")
	if !ok || ip != "127.0.0.1" || port != 8080 {
		t.Fatalf("%s %d %v", ip, port, ok)
	}
}
func TestServiceName(t *testing.T) {
	name, k := serviceName("java", "java\x00-Dweblogic.Name=Core01\x00")
	if name != "Core01" || k != "java" {
		t.Fatalf("%s %s", name, k)
	}
}
func TestDetect(t *testing.T) {
	d, e := Detect()
	if e != nil || d.HostID == "" || len(d.Processes) == 0 {
		t.Fatalf("%+v %v", d, e)
	}
}
