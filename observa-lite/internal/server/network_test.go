package server

import (
	"encoding/json"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeDeviceCredentialsAndTemplatePreview(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, e := store.NewSQL(path, "sqlite", "")
	if e != nil {
		t.Fatal(e)
	}
	key := "admin-native-snmp-test-token-12345"
	s := New(st, key, "agent-native-snmp-test-token-12345", true)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-API-Key", key)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	doc := `{"zabbix_export":{"version":"7.0","templates":[{"name":"Test","template":"Test","items":[{"name":"Name","key":"system.name","type":"SNMP_AGENT","snmp_oid":"get[1.3.6.1.2.1.1.5.0]"}]}]}}`
	payload, _ := json.Marshal(map[string]any{"document": doc, "preview": true})
	w := call("POST", "/api/v1/network/templates/import", string(payload))
	if w.Code != 200 || len(st.NetworkTemplates()) != 0 {
		t.Fatalf("preview mutated %d %s", w.Code, w.Body.String())
	}
	payload, _ = json.Marshal(map[string]any{"document": doc, "preview": false})
	w = call("POST", "/api/v1/network/templates/import", string(payload))
	if w.Code != 200 || len(st.NetworkTemplates()) != 1 {
		t.Fatal(w.Body.String())
	}
	body := `{"id":"switch","name":"Core","kind":"snmp","enabled":true,"target":"10.0.0.1","snmp_version":"2c","snmp_port":161,"template_id":"generic-snmp","credentials":{"community":"controlled-sensitive-community"}}`
	if w = call("POST", "/api/v1/integrations", body); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	saved, _ := st.Integration("switch")
	if saved.SNMPRevision == "" {
		t.Fatal("missing configuration revision")
	}
	w = call("GET", "/api/v1/integrations", "")
	if strings.Contains(w.Body.String(), "controlled-sensitive-community") || strings.Contains(w.Body.String(), "integration_secrets") {
		t.Fatal("secret exposed")
	}
	backup, e := st.ExportJSON()
	if e != nil || strings.Contains(string(backup), "controlled-sensitive-community") {
		t.Fatal("unencrypted backup")
	}
	cred, e := st.SNMPCredentials(key, "switch")
	if e != nil || cred.Community != "controlled-sensitive-community" {
		t.Fatal("secret unavailable")
	}
	if _, e = st.SNMPCredentials("wrong-key", "switch"); e == nil {
		t.Fatal("wrong key accepted")
	}
	// A normal edit preserves the stored secret; no decrypted data is returned to the form.
	if w = call("POST", "/api/v1/integrations", `{"id":"switch","name":"Renamed","kind":"snmp","enabled":true,"target":"10.0.0.1","snmp_version":"2c","template_id":"generic-snmp"}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	cred, e = st.SNMPCredentials(key, "switch")
	if e != nil || cred.Community != "controlled-sensitive-community" {
		t.Fatal("edit lost secret")
	}
	reopened, e := store.NewSQL(path, "sqlite", "")
	if e != nil {
		t.Fatal(e)
	}
	cred, e = reopened.SNMPCredentials(key, "switch")
	if e != nil || cred.Community == "" || len(reopened.NetworkTemplates()) != 1 {
		t.Fatal("SQLite restart lost credentials or templates")
	}
	if w = call("DELETE", "/api/v1/integrations/switch", ""); w.Code != 200 || st.HasSNMPCredentials("switch") {
		t.Fatal("orphaned credentials")
	}
}
