package network

import (
	"errors"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"regexp"
	"strings"
)

func compileDependentItem(raw rawItem, macros map[string]string, prototype bool, masters map[string]bool) model.NetworkItem {
	if raw.Type == "DEPENDENT" || raw.Type == "18" {
		if masters[raw.Master.Key] && len(raw.Preprocessing) > 0 {
			step := raw.Preprocessing[0]
			if (step.Type == "SNMP_WALK_VALUE" || step.Type == "28") && len(step.Parameters) == 2 && step.Parameters[1] == "0" && (step.ErrorHandler == "" || step.ErrorHandler == "DISCARD_VALUE") {
				raw.OID = step.Parameters[0]
				raw.Type = "SNMP_AGENT"
				raw.Preprocessing = raw.Preprocessing[1:]
			}
		}
	}
	return item(raw, macros, prototype)
}
func compileDiscovery(raw rawDiscovery, macros map[string]string, masters map[string]bool) model.NetworkDiscovery {
	d := model.NetworkDiscovery{Name: raw.Name, Columns: map[string]string{}, Items: []model.NetworkItem{}, Supported: true, FilterMode: raw.Filter.EvalType, Conditions: []model.NetworkCondition{}}
	oid := expand(raw.OID, macros)
	if raw.Type == "DEPENDENT" || raw.Type == "18" {
		if !masters[raw.Master.Key] {
			d.Reason = "Master SNMP walk ausente ou incompatível"
		} else {
			converted := false
			for _, step := range raw.Preprocessing {
				switch step.Type {
				case "SNMP_WALK_TO_JSON", "29":
					if converted || len(step.Parameters)%3 != 0 || len(step.Parameters) == 0 || len(step.Parameters) > 48 {
						d.Reason = "Colunas SNMP_WALK_TO_JSON inválidas"
						continue
					}
					converted = true
					for i := 0; i < len(step.Parameters); i += 3 {
						macro, base, format := step.Parameters[i], expand(step.Parameters[i+1], macros), step.Parameters[i+2]
						if !strings.HasPrefix(macro, "{#") || !strings.HasSuffix(macro, "}") || !NumericOID(base) || format != "0" {
							d.Reason = "Coluna/formato SNMP_WALK_TO_JSON incompatível"
						}
						d.Columns[macro] = base
					}
				case "DISCARD_UNCHANGED_HEARTBEAT", "DISCARD_UNCHANGED", "19", "20":
				default:
					d.Reason = "Pré-processamento LLD não suportado: " + step.Type
				}
				if step.ErrorHandler != "" && step.ErrorHandler != "DISCARD_VALUE" {
					d.Reason = "Tratamento de erro LLD não suportado"
				}
			}
			if !converted {
				d.Reason = "LLD dependente exige SNMP_WALK_TO_JSON"
			}
		}
	} else if (raw.Type == "SNMP_AGENT" || raw.Type == "20" || raw.Type == "4") && strings.HasPrefix(oid, "discovery[") && strings.HasSuffix(oid, "]") {
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
	} else {
		d.Reason = "Descoberta SNMP não executável"
	}
	if len(raw.Filter.Conditions) > 32 {
		d.Reason = "Limite de 32 condições LLD"
	}
	for _, condition := range raw.Filter.Conditions {
		pattern := expand(condition.Value, macros)
		negate := condition.Operator == "NOT_MATCHES_REGEX" || condition.Operator == "9"
		if condition.Operator != "" && condition.Operator != "MATCHES_REGEX" && condition.Operator != "8" && !negate {
			d.Reason = "Operador de filtro LLD não suportado"
		}
		if _, ok := d.Columns[condition.Macro]; !ok && condition.Macro != "{#SNMPINDEX}" {
			d.Reason = "Filtro LLD sem coluna de origem"
		}
		if len(pattern) > 4096 {
			d.Reason = "Expressão de filtro excessiva"
		}
		if strings.Contains(pattern, "{$") {
			d.Reason = "Macro de filtro LLD não resolvida"
		}
		if _, e := regexp.Compile(pattern); e != nil {
			d.Reason = "Regex LLD incompatível com RE2"
		}
		d.Conditions = append(d.Conditions, model.NetworkCondition{Macro: condition.Macro, Pattern: pattern, Negate: negate})
	}
	if _, e := CompileDiscoveryFilter(d); e != nil {
		d.Reason = e.Error()
	}
	d.Supported = d.Reason == ""
	return d
}
func CompileDiscoveryFilter(d model.NetworkDiscovery) (func(map[string]string) bool, error) {
	mode := d.FilterMode
	switch mode {
	case "", "0", "AND_OR":
		mode = "AND_OR"
	case "1", "AND":
		mode = "AND"
	case "2", "OR":
		mode = "OR"
	default:
		return nil, errors.New("Fórmula de filtro LLD não suportada")
	}
	patterns := []*regexp.Regexp{}
	for _, condition := range d.Conditions {
		r, e := regexp.Compile(condition.Pattern)
		if e != nil {
			return nil, e
		}
		patterns = append(patterns, r)
	}
	return func(row map[string]string) bool {
		if len(d.Conditions) == 0 {
			return true
		}
		groups := map[string]bool{}
		for i, c := range d.Conditions {
			value, present := row[c.Macro]
			hit := present && patterns[i].MatchString(value)
			if present && c.Negate {
				hit = !hit
			}
			switch mode {
			case "AND":
				if !hit {
					return false
				}
			case "OR":
				if hit {
					return true
				}
			default:
				groups[c.Macro] = groups[c.Macro] || hit
			}
		}
		if mode == "OR" {
			return false
		}
		for _, hit := range groups {
			if !hit {
				return false
			}
		}
		return true
	}, nil
}
