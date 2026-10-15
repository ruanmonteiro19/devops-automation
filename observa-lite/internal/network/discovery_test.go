package network

import (
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"testing"
)

const modern = `zabbix_export:
  version: '7.0'
  templates:
    - template: Modern
      name: Modern switch
      items:
        - name: Walk
          key: interfaces.walk
          type: SNMP_AGENT
          snmp_oid: 'walk[1.3.6.1.2.1.31.1.1.1.1,1.3.6.1.2.1.2.2.1.8]'
      discovery_rules:
        - name: Interfaces
          type: DEPENDENT
          master_item:
            key: interfaces.walk
          preprocessing:
            - type: SNMP_WALK_TO_JSON
              parameters: ['{#IFNAME}', '1.3.6.1.2.1.31.1.1.1.1', '0']
          filter:
            evaltype: AND
            conditions:
              - macro: '{#IFNAME}'
                value: '^Gi'
          item_prototypes:
            - name: Status
              key: 'interface[{#SNMPINDEX}]'
              type: DEPENDENT
              master_item:
                key: interfaces.walk
              preprocessing:
                - type: SNMP_WALK_VALUE
                  parameters: ['1.3.6.1.2.1.2.2.1.8.{#SNMPINDEX}', '0']
`

func TestModernSNMPDiscoveryCompilation(t *testing.T) {
	templates, e := ParseTemplates([]byte(modern))
	if e != nil {
		t.Fatal(e)
	}
	d := templates[0].Discovery[0]
	if !d.Supported || !d.Items[0].Supported {
		t.Fatalf("%+v", d)
	}
	match, e := CompileDiscoveryFilter(d)
	if e != nil || !match(map[string]string{"{#IFNAME}": "Gi1"}) || match(map[string]string{"{#IFNAME}": "Loopback0"}) {
		t.Fatal("filter semantics")
	}
	if d.Items[0].OID != "1.3.6.1.2.1.2.2.1.8.{#SNMPINDEX}" {
		t.Fatal("dependent OID not compiled")
	}
}
func TestDiscoveryBooleanAndPCRELimits(t *testing.T) {
	d := model.NetworkDiscovery{FilterMode: "AND_OR", Conditions: []model.NetworkCondition{{Macro: "type", Pattern: "^1$"}, {Macro: "type", Pattern: "^2$"}, {Macro: "name", Pattern: "^lo", Negate: true}}}
	match, e := CompileDiscoveryFilter(d)
	if e != nil || !match(map[string]string{"type": "2", "name": "eth0"}) || match(map[string]string{"type": "2", "name": "lo0"}) || match(map[string]string{"type": "3", "name": "eth0"}) {
		t.Fatal("AND/OR grouping invalid")
	}
	d.Conditions[0].Pattern = "(?=unsupported)"
	if _, e := CompileDiscoveryFilter(d); e == nil {
		t.Fatal("PCRE lookahead accepted")
	}
}
