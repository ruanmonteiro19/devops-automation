package collectors

import (
	"context"
	"github.com/gosnmp/gosnmp"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	networkmodel "github.com/ruanmonteiro19/observa-lite/internal/network"
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
			data := []gosnmp.SnmpPDU{
				{Name: ".1.0.8802.1.1.2.1.3.7.1.3.99", Type: gosnmp.OctetString, Value: "Gi1/0/1"},
				{Name: ".1.0.8802.1.1.2.1.4.1.1.5.0.99.1", Type: gosnmp.OctetString, Value: []byte{0, 1, 2, 3, 4, 5}},
				{Name: ".1.0.8802.1.1.2.1.4.1.1.7.0.99.1", Type: gosnmp.OctetString, Value: "eth0"},
				{Name: ".1.0.8802.1.1.2.1.4.1.1.9.0.99.1", Type: gosnmp.OctetString, Value: "Application host"},
				{Name: ".1.0.8802.1.1.2.1.4.2.1.3.0.99.1.1.4.10.0.0.20", Type: gosnmp.Integer, Value: 2}, {Name: ".1.3.6.1.2.1.1.1.0", Type: gosnmp.OctetString, Value: "Controlled switch"}, {Name: ".1.3.6.1.2.1.1.2.0", Type: gosnmp.ObjectIdentifier, Value: ".1.3.6.1.4.1.9"}, {Name: ".1.3.6.1.2.1.1.3.0", Type: gosnmp.TimeTicks, Value: uint32(12345)}, {Name: ".1.3.6.1.2.1.1.5.0", Type: gosnmp.OctetString, Value: "Fixture switch"}, {Name: ".1.3.6.1.2.1.2.2.1.8.1", Type: gosnmp.Integer, Value: 1}, {Name: ".1.3.6.1.2.1.31.1.1.1.1.1", Type: gosnmp.OctetString, Value: "Gi1/0/1"}, {Name: ".1.3.6.1.2.1.31.1.1.1.6.1", Type: gosnmp.Counter64, Value: uint64(123456789)}, {Name: ".1.3.6.1.2.1.31.1.1.1.15.1", Type: gosnmp.Gauge32, Value: uint32(1000)}}
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
				templates, e := networkmodel.ParseTemplates([]byte(`{"zabbix_export":{"version":"7.0","templates":[{"template":"Modern","name":"Modern switch","items":[{"name":"Uptime","key":"uptime.seconds","type":"SNMP_AGENT","snmp_oid":"get[1.3.6.1.2.1.1.3.0]","preprocessing":[{"type":"MULTIPLIER","parameters":["0.01"]}]},{"name":"Walk","key":"interfaces.walk","type":"SNMP_AGENT","snmp_oid":"walk[1.3.6.1.2.1.31.1.1.1.1,1.3.6.1.2.1.2.2.1.8]"}],"discovery_rules":[{"name":"Interfaces","type":"DEPENDENT","master_item":{"key":"interfaces.walk"},"preprocessing":[{"type":"SNMP_WALK_TO_JSON","parameters":["{#IFNAME}","1.3.6.1.2.1.31.1.1.1.1","0"]}],"filter":{"evaltype":"AND","conditions":[{"macro":"{#IFNAME}","value":"^Gi"}]},"item_prototypes":[{"name":"Status","key":"interface[{#SNMPINDEX}]","type":"DEPENDENT","master_item":{"key":"interfaces.walk"},"preprocessing":[{"type":"SNMP_WALK_VALUE","parameters":["1.3.6.1.2.1.2.2.1.8.{#SNMPINDEX}","0"]}]}]}]}]}}`))
				if e != nil {
					t.Fatal(e)
				}
				return templates[0], true
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result, e := runner.CollectIntegration(ctx, model.Integration{ID: "switch", Name: "Switch", Kind: "snmp", Target: "127.0.0.1", SNMPVersion: version, SNMPPort: uint16(conn.LocalAddr().(*net.UDPAddr).Port), TemplateID: "fixture-template", NeighborDiscovery: true})
			if e != nil {
				t.Fatal(e)
			}
			if len(result.Resources) != 1 || result.Resources[0].Name != "Fixture switch" || len(result.Resources[0].Interfaces) != 1 || result.Resources[0].Interfaces[0].Name != "Gi1/0/1" {
				t.Fatalf("unexpected %+v", result)
			}
			if result.Resources[0].Metrics["uptime.seconds"] != 123.45 || result.Resources[0].Metrics["interface[1]"] != 1 {
				t.Fatalf("template/LLD not executed: %+v", result.Resources[0].Metrics)
			}
			if len(result.Resources[0].Neighbors) != 1 || result.Resources[0].Neighbors[0].LocalIfIndex != "1" || result.Resources[0].Neighbors[0].RemoteIP != "10.0.0.20" {
				t.Fatalf("invalid LLDP neighbor: %+v", result.Resources[0].Neighbors)
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
