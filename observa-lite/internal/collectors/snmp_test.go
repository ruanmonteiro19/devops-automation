package collectors

import (
	"context"
	"github.com/gosnmp/gosnmp"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// A real UDP SNMP agent fixture exercises BER encoding, requests, bulk walking and credentials.
func TestNativeSNMPWire(t *testing.T) {
	for _, version := range []string{"1", "2c"} {
		t.Run(version, func(t *testing.T) {
			conn, e := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
			if e != nil {
				t.Fatal(e)
			}
			var wg sync.WaitGroup
			wg.Add(1)
			t.Cleanup(func() { conn.Close(); wg.Wait() })
			data := []gosnmp.SnmpPDU{{Name: ".1.3.6.1.2.1.1.1.0", Type: gosnmp.OctetString, Value: "Controlled switch"}, {Name: ".1.3.6.1.2.1.1.2.0", Type: gosnmp.ObjectIdentifier, Value: ".1.3.6.1.4.1.9"}, {Name: ".1.3.6.1.2.1.1.3.0", Type: gosnmp.TimeTicks, Value: uint32(12345)}, {Name: ".1.3.6.1.2.1.1.5.0", Type: gosnmp.OctetString, Value: "Fixture switch"}, {Name: ".1.3.6.1.2.1.2.2.1.8.1", Type: gosnmp.Integer, Value: 1}, {Name: ".1.3.6.1.2.1.31.1.1.1.1.1", Type: gosnmp.OctetString, Value: "Gi1/0/1"}, {Name: ".1.3.6.1.2.1.31.1.1.1.6.1", Type: gosnmp.Counter64, Value: uint64(123456789)}, {Name: ".1.3.6.1.2.1.31.1.1.1.15.1", Type: gosnmp.Gauge32, Value: uint32(1000)}}
			less := func(a, b string) bool {
				aa, bb := strings.Split(strings.TrimPrefix(a, "."), "."), strings.Split(strings.TrimPrefix(b, "."), ".")
				for i := 0; i < len(aa) && i < len(bb); i++ {
					x, _ := strconv.Atoi(aa[i])
					y, _ := strconv.Atoi(bb[i])
					if x != y {
						return x < y
					}
				}
				return len(aa) < len(bb)
			}
			sort.Slice(data, func(i, j int) bool { return less(data[i].Name, data[j].Name) })
			go func() {
				defer wg.Done()
				buffer := make([]byte, 65535)
				decoder := &gosnmp.GoSNMP{}
				for {
					n, remote, err := conn.ReadFromUDP(buffer)
					if err != nil {
						return
					}
					packet, err := decoder.SnmpDecodePacket(buffer[:n])
					if err != nil || packet.Community != "fixture-community" {
						continue
					}
					vars := []gosnmp.SnmpPDU{}
					for _, oid := range packet.Variables {
						found := false
						for _, v := range data {
							if packet.PDUType == gosnmp.GetRequest && v.Name == oid.Name || packet.PDUType != gosnmp.GetRequest && less(oid.Name, v.Name) {
								vars = append(vars, v)
								found = true
								break
							}
						}
						if !found {
							kind := gosnmp.NoSuchInstance
							if packet.PDUType != gosnmp.GetRequest {
								kind = gosnmp.EndOfMibView
							}
							vars = append(vars, gosnmp.SnmpPDU{Name: oid.Name, Type: kind, Value: nil})
						}
					}
					response := gosnmp.SnmpPacket{Version: packet.Version, Community: packet.Community, PDUType: gosnmp.GetResponse, RequestID: packet.RequestID, Variables: vars}
					encoded, err := response.MarshalMsg()
					if err == nil {
						_, _ = conn.WriteToUDP(encoded, remote)
					}
				}
			}()
			runner := Runner{AllowPrivate: true, SNMPCredentials: func(string) (model.SNMPCredentials, error) {
				return model.SNMPCredentials{Community: "fixture-community"}, nil
			}}
			runner.NetworkTemplate = func(string) (model.NetworkTemplate, bool) {
				return model.NetworkTemplate{ID: "fixture-template", Items: []model.NetworkItem{{Name: "Uptime", Key: "uptime.seconds", OID: "1.3.6.1.2.1.1.3.0", Multiplier: .01, Supported: true}}, Discovery: []model.NetworkDiscovery{{Name: "Interfaces", Supported: true, Columns: map[string]string{"{#IFNAME}": "1.3.6.1.2.1.31.1.1.1.1"}, Items: []model.NetworkItem{{Name: "Status", Key: "interface[{#SNMPINDEX}]", OID: "1.3.6.1.2.1.2.2.1.8.{#SNMPINDEX}", Multiplier: 1, Supported: true}}}}}, true
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result, e := runner.CollectIntegration(ctx, model.Integration{ID: "switch", Name: "Switch", Kind: "snmp", Target: "127.0.0.1", SNMPVersion: version, SNMPPort: uint16(conn.LocalAddr().(*net.UDPAddr).Port), TemplateID: "fixture-template"})
			if e != nil {
				t.Fatal(e)
			}
			if len(result.Resources) != 1 || result.Resources[0].Name != "Fixture switch" || len(result.Resources[0].Interfaces) != 1 || result.Resources[0].Interfaces[0].Name != "Gi1/0/1" {
				t.Fatalf("unexpected %+v", result)
			}
			if result.Resources[0].Metrics["uptime.seconds"] != 123.45 || result.Resources[0].Metrics["interface[1]"] != 1 {
				t.Fatalf("template/LLD not executed: %+v", result.Resources[0].Metrics)
			}
			// Same literal private target must be refused before any UDP request.
			runner.AllowPrivate = false
			if _, e = runner.CollectIntegration(ctx, model.Integration{ID: "switch", Kind: "snmp", Target: "127.0.0.1", SNMPVersion: version}); e == nil {
				t.Fatal("private target allowed")
			}
		})
	}
}
func TestSNMPv3Configuration(t *testing.T) {
	ctx := context.Background()
	for _, level := range []string{"noAuthNoPriv", "authNoPriv", "authPriv"} {
		s := newSNMP(ctx, model.Integration{Target: "127.0.0.1", SNMPVersion: "3"}, model.SNMPCredentials{Username: "user", SecurityLevel: level, AuthProtocol: "SHA256", AuthPassword: "password1", PrivacyProtocol: "AES", PrivacyPassword: "password2"})
		if s.Version != gosnmp.Version3 || s.SecurityModel != gosnmp.UserSecurityModel {
			t.Fatal("invalid version")
		}
		if level == "authPriv" && s.MsgFlags != gosnmp.AuthPriv {
			t.Fatal("privacy missing")
		}
	}
}

func TestTemplateCounterRatesSkipFirstAndReset(t *testing.T) {
	now := time.Now()
	previous := model.IntegrationResult{SuccessAt: now.Add(-10 * time.Second), Resources: []model.IntegrationResource{{ID: "switch", Metrics: map[string]float64{"packets.raw_counter": 100}}}}
	current := model.IntegrationResult{SuccessAt: now, Resources: []model.IntegrationResource{{ID: "switch", Metrics: map[string]float64{"packets.raw_counter": 200}, Attributes: map[string]string{"preprocessing:packets": "per_second"}}}}
	DeviceRates(&current, previous)
	if current.Resources[0].Metrics["packets"] != 10 {
		t.Fatal(current.Resources[0].Metrics)
	}
	delete(current.Resources[0].Metrics, "packets")
	current.Resources[0].Metrics["packets.raw_counter"] = 50
	DeviceRates(&current, previous)
	if _, ok := current.Resources[0].Metrics["packets"]; ok {
		t.Fatal("counter reset produced a rate")
	}
	current.Resources[0].Attributes["preprocessing:packets"] = "delta"
	DeviceRates(&current, previous)
	if current.Resources[0].Metrics["packets"] != -50 {
		t.Fatal("simple change lost negative delta")
	}
}
