package network

import (
	"os"
	"strings"
	"testing"
)

const fixture = `zabbix_export:
  version: '7.0'
  templates:
    - template: Switch
      name: Switch by SNMP
      macros:
        - macro: '{$BASE}'
          value: '1.3.6.1.2.1'
      items:
        - name: Uptime
          type: SNMP_AGENT
          key: uptime
          snmp_oid: 'get[{$BASE}.1.3.0]'
          preprocessing:
            - type: MULTIPLIER
              parameters: ['0.01']
        - name: Unsafe JS
          type: SNMP_AGENT
          key: js
          snmp_oid: 1.3.6.1.2.1.1.3.0
          preprocessing:
            - type: JAVASCRIPT
              parameters: ['while(true){}']
      discovery_rules:
        - name: Interfaces
          type: SNMP_AGENT
          snmp_oid: 'discovery[{#IFNAME},1.3.6.1.2.1.31.1.1.1.1]'
          item_prototypes:
            - name: 'Interface {#IFNAME}'
              key: 'interface[{#SNMPINDEX}]'
              type: SNMP_AGENT
              snmp_oid: '1.3.6.1.2.1.2.2.1.8.{#SNMPINDEX}'
`

func TestImportAndReport(t *testing.T) {
	got, e := ParseTemplates([]byte(fixture))
	if e != nil {
		t.Fatal(e)
	}
	v := got[0]
	if v.Supported != 2 || v.Unsupported != 1 || v.Items[0].OID != "1.3.6.1.2.1.1.3.0" || v.Items[0].Multiplier != .01 || !strings.Contains(v.Items[1].Reason, "JAVASCRIPT") {
		t.Fatalf("unexpected: %+v", v)
	}
}
func TestXMLJSONAndRejectAliases(t *testing.T) {
	for _, doc := range []string{`{"zabbix_export":{"version":"7.0","templates":[{"template":"T","name":"T","items":[{"name":"Name","key":"name","type":"SNMP_AGENT","snmp_oid":"1.3.6.1.2.1.1.5.0"}]}]}}`, `<zabbix_export><version>6.0</version><templates><template><template>T</template><name>T</name><items><item><name>Name</name><key>name</key><type>20</type><snmp_oid>1.3.6.1.2.1.1.5.0</snmp_oid></item></items></template></templates></zabbix_export>`} {
		v, e := ParseTemplates([]byte(doc))
		if e != nil || v[0].Supported != 1 {
			t.Fatalf("%+v %v", v, e)
		}
	}
	if _, e := ParseTemplates([]byte("zabbix_export: &x {templates: [*x]}")); e == nil {
		t.Fatal("alias accepted")
	}
}
func TestOfficialTemplateWhenProvided(t *testing.T) {
	path := os.Getenv("VETRA_ZABBIX_FIXTURE")
	if path == "" {
		t.Skip("set VETRA_ZABBIX_FIXTURE to validate an upstream template")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	v, e := ParseTemplates(b)
	if e != nil {
		t.Fatal(e)
	}
	if len(v) != 1 || v[0].Supported < 10 || v[0].Unsupported == 0 {
		t.Fatalf("expected useful partial compatibility: %+v", v)
	}
	t.Logf("Official %s: supported=%d unsupported=%d warnings=%v", v[0].Name, v[0].Supported, v[0].Unsupported, v[0].Warnings)
}
