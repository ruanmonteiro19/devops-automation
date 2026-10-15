// Package network imports a bounded, explicitly reported subset of Zabbix exports.
// It never executes scripts, expressions, commands or arbitrary JavaScript.
package network

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"gopkg.in/yaml.v3"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type rawStep struct {
	Type       string   `yaml:"type" xml:"type"`
	Parameters []string `yaml:"parameters" xml:"parameters>parameter"`
}
type rawItem struct {
	Name          string     `yaml:"name" xml:"name"`
	Key           string     `yaml:"key" xml:"key"`
	Type          string     `yaml:"type" xml:"type"`
	OID           string     `yaml:"snmp_oid" xml:"snmp_oid"`
	Units         string     `yaml:"units" xml:"units"`
	Preprocessing []rawStep  `yaml:"preprocessing" xml:"preprocessing>step"`
	Triggers      []struct{} `yaml:"triggers" xml:"triggers>trigger"`
}
type rawDiscovery struct {
	TriggerPrototypes []struct{} `yaml:"trigger_prototypes" xml:"trigger_prototypes>trigger_prototype"`
	GraphPrototypes   []struct{} `yaml:"graph_prototypes" xml:"graph_prototypes>graph_prototype"`
	HostPrototypes    []struct{} `yaml:"host_prototypes" xml:"host_prototypes>host_prototype"`
	Name              string     `yaml:"name" xml:"name"`
	Type              string     `yaml:"type" xml:"type"`
	OID               string     `yaml:"snmp_oid" xml:"snmp_oid"`
	Items             []rawItem  `yaml:"item_prototypes" xml:"item_prototypes>item_prototype"`
	Filter            struct {
		Conditions []struct{} `yaml:"conditions" xml:"conditions>condition"`
	} `yaml:"filter" xml:"filter"`
}
type rawTemplate struct {
	HTTPTests  []struct{}     `yaml:"httptests" xml:"httptests>httptest"`
	ValueMaps  []struct{}     `yaml:"valuemaps" xml:"valuemaps>valuemap"`
	Dashboards []struct{}     `yaml:"dashboards" xml:"dashboards>dashboard"`
	Name       string         `yaml:"name" xml:"name"`
	Template   string         `yaml:"template" xml:"template"`
	Items      []rawItem      `yaml:"items" xml:"items>item"`
	Discovery  []rawDiscovery `yaml:"discovery_rules" xml:"discovery_rules>discovery_rule"`
	Macros     []struct {
		Macro string `yaml:"macro" xml:"macro"`
		Value string `yaml:"value" xml:"value"`
	} `yaml:"macros" xml:"macros>macro"`
	Linked   []struct{} `yaml:"templates" xml:"templates>template"`
	Triggers []struct{} `yaml:"triggers" xml:"triggers>trigger"`
	Graphs   []struct{} `yaml:"graphs" xml:"graphs>graph"`
}
type export struct {
	Version   string        `yaml:"version" xml:"version"`
	Templates []rawTemplate `yaml:"templates" xml:"templates>template"`
	Triggers  []struct{}    `yaml:"triggers" xml:"triggers>trigger"`
	Graphs    []struct{}    `yaml:"graphs" xml:"graphs>graph"`
}

var oidPattern = regexp.MustCompile(`^\.?[0-9]+(?:\.[0-9]+)+$`)
var secretMacro = regexp.MustCompile(`(?i)COMMUNITY|PASSWORD|PASSPHRASE|SECRET|TOKEN`)

func NumericOID(oid string) bool { return len(oid) <= 256 && oidPattern.MatchString(oid) }
func expand(s string, m map[string]string) string {
	for i := 0; i < 8; i++ {
		old := s
		for k, v := range m {
			if len(s)+strings.Count(s, k)*(len(v)-len(k)) > 4096 {
				return "!macro-expansion-limit"
			}
			s = strings.ReplaceAll(s, k, v)
		}
		if old == s {
			break
		}
	}
	return s
}
func item(v rawItem, macros map[string]string, prototype bool) model.NetworkItem {
	out := model.NetworkItem{Name: v.Name, Key: v.Key, OID: expand(strings.TrimSpace(v.OID), macros), Units: v.Units, Multiplier: 1, Supported: true}
	if v.Type != "SNMP_AGENT" && v.Type != "20" && v.Type != "SNMPV2" && v.Type != "4" && v.Type != "1" && v.Type != "6" {
		out.Reason = "Tipo de item não executável: " + v.Type
	}
	if strings.HasPrefix(out.OID, "get[") && strings.HasSuffix(out.OID, "]") {
		out.OID = strings.TrimSuffix(strings.TrimPrefix(out.OID, "get["), "]")
	}
	testOID := out.OID
	if prototype {
		testOID = strings.ReplaceAll(testOID, "{#SNMPINDEX}", "1")
	}
	if !NumericOID(testOID) {
		out.Reason = "OID precisa ser numérico e escalar; walk/get, MIB simbólica ou macro não resolvida"
	}
	for _, step := range v.Preprocessing {
		switch step.Type {
		case "CHANGE_PER_SECOND", "10", "SIMPLE_CHANGE", "9":
			if out.Change != "" {
				out.Reason = "Múltiplas transformações de diferença não suportadas"
			}
			if step.Type == "CHANGE_PER_SECOND" || step.Type == "10" {
				out.Change = "per_second"
			} else {
				out.Change = "delta"
			}
		case "DISCARD_UNCHANGED", "19", "DISCARD_UNCHANGED_HEARTBEAT", "20":
		// Vetra stores latest inventory values, not a Zabbix per-item history stream.
		case "MULTIPLIER", "1":
			if len(step.Parameters) != 1 {
				out.Reason = "Multiplicador inválido"
				continue
			}
			n, e := strconv.ParseFloat(step.Parameters[0], 64)
			if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || n > 1e15 || n < -1e15 {
				out.Reason = "Multiplicador inválido"
			} else {
				out.Multiplier *= n
				if math.IsInf(out.Multiplier, 0) || math.IsNaN(out.Multiplier) {
					out.Reason = "Multiplicador acumulado inválido"
				}
			}
		default:
			out.Reason = "Pré-processamento não suportado: " + step.Type
		}
	}
	if len(v.Triggers) > 0 { /* Trigger expressions are reported on the template, never executed. */
	}
	if len(out.Name) > 256 || len(out.Key) > 256 || out.Key == "" {
		out.Reason = "Nome/chave inválidos"
	}
	out.Supported = out.Reason == ""
	return out
}
func ParseTemplates(data []byte) ([]model.NetworkTemplate, error) {
	if len(data) == 0 || len(data) > 2<<20 {
		return nil, errors.New("arquivo deve ter entre 1 byte e 2 MiB")
	}
	var doc export
	if strings.HasPrefix(strings.TrimSpace(string(data)), "<") {
		if e := xml.Unmarshal(data, &doc); e != nil {
			return nil, errors.New("XML de template inválido")
		}
	} else {
		// Decode YAML into a node first to reject anchors/aliases and expansion bombs.
		var root yaml.Node
		if e := yaml.Unmarshal(data, &root); e != nil {
			return nil, errors.New("YAML/JSON inválido")
		}
		count := 0
		var check func(*yaml.Node, int) error
		check = func(n *yaml.Node, depth int) error {
			count++
			if count > 80000 || depth > 35 || n.Kind == yaml.AliasNode || n.Anchor != "" {
				return errors.New("aliases, profundidade ou tamanho do documento não permitidos")
			}
			for _, c := range n.Content {
				if e := check(c, depth+1); e != nil {
					return e
				}
			}
			return nil
		}
		if e := check(&root, 0); e != nil {
			return nil, e
		}
		var wrapper struct {
			Export export `yaml:"zabbix_export"`
		}
		if e := root.Decode(&wrapper); e != nil {
			return nil, errors.New("estrutura de exportação inválida")
		}
		doc = wrapper.Export
	}
	if len(doc.Templates) == 0 || len(doc.Templates) > 50 {
		return nil, errors.New("exportação precisa conter entre 1 e 50 templates")
	}
	out := []model.NetworkTemplate{}
	hash := sha256.Sum256(data)
	for _, raw := range doc.Templates {
		name := raw.Name
		if name == "" {
			name = raw.Template
		}
		if name == "" || len(name) > 256 {
			return nil, errors.New("nome de template inválido")
		}
		id := sha256.Sum256([]byte(raw.Template + "\x00" + name))
		t := model.NetworkTemplate{ID: "zbx-" + hex.EncodeToString(id[:12]), Name: name, Version: doc.Version, SourceSHA: hex.EncodeToString(hash[:]), Items: []model.NetworkItem{}, Discovery: []model.NetworkDiscovery{}, Warnings: []string{"Intervalos, histórico, tendências e supressão de repetidos do Zabbix não são reproduzidos; este coletor usa o intervalo do perfil e a retenção nativa do Vetra."}}
		if len(raw.Macros) > 200 {
			return nil, errors.New("limite de 200 macros por template")
		}
		macros := map[string]string{}
		for _, m := range raw.Macros {
			if len(m.Macro) > 128 || !strings.HasPrefix(m.Macro, "{$") || !strings.HasSuffix(m.Macro, "}") {
				continue
			}
			if secretMacro.MatchString(m.Macro) {
				t.Warnings = append(t.Warnings, "Macro de credencial ignorada: "+m.Macro)
				continue
			}
			if len(m.Value) <= 256 {
				macros[m.Macro] = m.Value
			}
		}
		if len(raw.Items) > 2000 || len(raw.Discovery) > 100 {
			return nil, errors.New("template excede 2000 itens ou 100 descobertas")
		}
		triggers := len(raw.Triggers) + len(doc.Triggers)
		for _, v := range raw.Items {
			t.Items = append(t.Items, item(v, macros, false))
			triggers += len(v.Triggers)
		}
		for _, r := range raw.Discovery {
			triggers += len(r.TriggerPrototypes)
			if len(r.GraphPrototypes) > 0 {
				t.Warnings = append(t.Warnings, "Gráficos de descoberta não importados: "+r.Name)
			}
			if len(r.HostPrototypes) > 0 {
				t.Warnings = append(t.Warnings, "Protótipos de host não executados: "+r.Name)
			}
			d := model.NetworkDiscovery{Name: r.Name, Columns: map[string]string{}, Items: []model.NetworkItem{}, Supported: true}
			oid := expand(r.OID, macros)
			if (r.Type != "SNMP_AGENT" && r.Type != "20" && r.Type != "4") || !strings.HasPrefix(oid, "discovery[") || !strings.HasSuffix(oid, "]") {
				d.Reason = "Descoberta exige discovery[macro,OID,...] SNMP numérica"
			} else {
				parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(oid, "discovery["), "]"), ",")
				if len(parts)%2 != 0 || len(parts) > 32 {
					d.Reason = "Colunas de descoberta inválidas"
				} else {
					for i := 0; i < len(parts); i += 2 {
						macro, base := strings.TrimSpace(parts[i]), strings.TrimSpace(parts[i+1])
						if !strings.HasPrefix(macro, "{#") || !strings.HasSuffix(macro, "}") || !NumericOID(base) {
							d.Reason = "Macro/OID de descoberta inválida"
						}
						d.Columns[macro] = base
					}
				}
			}
			if len(r.Filter.Conditions) > 0 {
				d.Reason = "Filtros LLD ainda não executáveis"
			}
			if len(r.Items) > 500 {
				return nil, errors.New("limite de 500 protótipos por descoberta")
			}
			for _, v := range r.Items {
				d.Items = append(d.Items, item(v, macros, true))
				triggers += len(v.Triggers)
			}
			d.Supported = d.Reason == ""
			t.Discovery = append(t.Discovery, d)
		}
		if len(raw.HTTPTests) > 0 {
			t.Warnings = append(t.Warnings, fmt.Sprintf("%d cenários HTTP não executados", len(raw.HTTPTests)))
		}
		if len(raw.ValueMaps) > 0 {
			t.Warnings = append(t.Warnings, "Mapeamentos de valores não aplicados; valores SNMP permanecem numéricos/brutos")
		}
		if len(raw.Dashboards) > 0 {
			t.Warnings = append(t.Warnings, "Dashboards Zabbix não importados")
		}
		if len(raw.Linked) > 0 {
			t.Warnings = append(t.Warnings, fmt.Sprintf("%d templates vinculados não expandidos; importe e configure separadamente", len(raw.Linked)))
		}
		if triggers > 0 {
			t.Warnings = append(t.Warnings, fmt.Sprintf("%d expressões de trigger não executadas", triggers))
		}
		if len(raw.Graphs)+len(doc.Graphs) > 0 {
			t.Warnings = append(t.Warnings, "Gráficos Zabbix não importados")
		}
		for _, v := range t.Items {
			if v.Supported {
				t.Supported++
			} else {
				t.Unsupported++
			}
		}
		for _, d := range t.Discovery {
			if !d.Supported {
				t.Unsupported += 1 + len(d.Items)
				continue
			}
			for _, v := range d.Items {
				if v.Supported {
					t.Supported++
				} else {
					t.Unsupported++
				}
			}
		}
		out = append(out, t)
	}
	return out, nil
}
func Builtin() model.NetworkTemplate {
	return model.NetworkTemplate{ID: "generic-snmp", Name: "SNMP padrão · Sistema e interfaces", Builtin: true, Items: []model.NetworkItem{}, Discovery: []model.NetworkDiscovery{}, Warnings: []string{"Disponibilidade de métricas depende das MIBs do equipamento. O perfil padrão lê SNMPv2-MIB, IF-MIB e IF-X-MIB."}}
}

// ValidateCredentials checks metadata without including secrets in error messages.
func ValidateCredentials(version string, c model.SNMPCredentials) error {
	b, _ := json.Marshal(c)
	if len(b) > 8192 {
		return errors.New("credenciais excedem o limite")
	}
	if version != "3" {
		if c.Community == "" || len(c.Community) > 255 {
			return errors.New("comunidade SNMP obrigatória (até 255 caracteres)")
		}
		return nil
	}
	if c.Username == "" || len(c.Username) > 64 {
		return errors.New("usuário SNMPv3 obrigatório")
	}
	if c.SecurityLevel != "noAuthNoPriv" && c.SecurityLevel != "authNoPriv" && c.SecurityLevel != "authPriv" {
		return errors.New("nível de segurança SNMPv3 inválido")
	}
	if c.SecurityLevel != "noAuthNoPriv" {
		if c.AuthProtocol != "SHA" && c.AuthProtocol != "SHA256" && c.AuthProtocol != "SHA512" {
			return errors.New("autenticação aceita SHA, SHA256 ou SHA512")
		}
		if len(c.AuthPassword) < 8 {
			return errors.New("senha de autenticação precisa ter ao menos 8 caracteres")
		}
	}
	if c.SecurityLevel == "authPriv" {
		if c.PrivacyProtocol != "AES" && c.PrivacyProtocol != "AES256" {
			return errors.New("privacidade aceita AES ou AES256")
		}
		if len(c.PrivacyPassword) < 8 {
			return errors.New("senha de privacidade precisa ter ao menos 8 caracteres")
		}
	}
	return nil
}
