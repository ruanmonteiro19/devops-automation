// Package vql implements a deliberately bounded pipeline language, not Dynatrace DQL.
package vql

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Row = map[string]any
type Result struct {
	Rows      []Row    `json:"rows"`
	Columns   []string `json:"columns"`
	Matched   int      `json:"matched"`
	Truncated bool     `json:"truncated"`
	Language  string   `json:"language"`
}

var field = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*$`)
var filter = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_.]*)\s*(==|!=|>=|<=|>|<|contains)\s*(.+)$`)
var aggregate = regexp.MustCompile(`^(count|avg|sum|min|max)\(([a-zA-Z0-9_.]*)\)$`)

func Split(s string) ([]string, error) {
	if len(s) > 8192 {
		return nil, fmt.Errorf("consulta excede 8192 caracteres")
	}
	parts := []string{}
	start := 0
	quoted, escaped := false, false
	for i, c := range s {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && quoted {
			escaped = true
			continue
		}
		if c == '"' {
			quoted = !quoted
		}
		if c == '|' && !quoted {
			parts = append(parts, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	if quoted {
		return nil, fmt.Errorf("texto sem aspas de fechamento")
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	if len(parts) > 12 {
		return nil, fmt.Errorf("máximo de 12 etapas")
	}
	return parts, nil
}
func Source(query string) (string, []string, error) {
	p, e := Split(query)
	if e != nil {
		return "", nil, e
	}
	if len(p) == 0 || !strings.HasPrefix(p[0], "fetch ") {
		return "", nil, fmt.Errorf("comece com fetch <dataset>")
	}
	name := strings.TrimSpace(strings.TrimPrefix(p[0], "fetch "))
	switch name {
	case "hosts", "processes", "services", "spans", "logs", "metrics", "rum", "frontends", "databases", "vulnerabilities", "events":
	default:
		return "", nil, fmt.Errorf("dataset não suportado: %s", name)
	}
	return name, p[1:], nil
}
func get(r Row, key string) any {
	if v, ok := r[key]; ok {
		return v
	}
	var value any = r
	for _, k := range strings.Split(key, ".") {
		m, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = m[k]
	}
	return value
}
func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, e := n.Float64()
		return f, e == nil
	default:
		return 0, false
	}
}
func compare(a, b any) int {
	av, aok := number(a)
	bv, bok := number(b)
	if aok && bok {
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
		return 0
	}
	return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
}
func literal(raw string) (any, error) {
	if strings.HasPrefix(raw, "\"") {
		var s string
		e := json.Unmarshal([]byte(raw), &s)
		return s, e
	}
	if raw == "true" {
		return true, nil
	}
	if raw == "false" {
		return false, nil
	}
	if raw == "null" {
		return nil, nil
	}
	f, e := strconv.ParseFloat(raw, 64)
	if e != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, fmt.Errorf("valores de texto precisam de aspas duplas")
	}
	return f, nil
}
func Execute(query string, input []Row) (Result, error) {
	_, stages, e := Source(query)
	if e != nil {
		return Result{}, e
	}
	rows := append([]Row{}, input...)
	truncated := false
	if len(rows) > 15000 {
		rows = rows[:15000]
		truncated = true
	}
	limit := 500
	for _, stage := range stages {
		switch {
		case strings.HasPrefix(stage, "filter "):
			m := filter.FindStringSubmatch(strings.TrimPrefix(stage, "filter "))
			if m == nil {
				return Result{}, fmt.Errorf("filter inválido")
			}
			value, err := literal(m[3])
			if err != nil {
				return Result{}, err
			}
			next := []Row{}
			for _, r := range rows {
				actual := get(r, m[1])
				c := compare(actual, value)
				_, an := number(actual)
				_, bn := number(value)
				compatible := (an && bn) || reflect.TypeOf(actual) == reflect.TypeOf(value)
				equal := compatible && c == 0
				match := false
				switch m[2] {
				case "==":
					match = equal
				case "!=":
					match = !equal
				case ">":
					match = compatible && actual != nil && c > 0
				case "<":
					match = compatible && actual != nil && c < 0
				case ">=":
					match = compatible && actual != nil && c >= 0
				case "<=":
					match = compatible && actual != nil && c <= 0
				case "contains":
					av, aok := actual.(string)
					bv, bok := value.(string)
					match = aok && bok && strings.Contains(av, bv)
				}
				if match {
					next = append(next, r)
				}
			}
			rows = next
		case strings.HasPrefix(stage, "fields "):
			keys := strings.Split(strings.TrimPrefix(stage, "fields "), ",")
			for _, key := range keys {
				if !field.MatchString(strings.TrimSpace(key)) {
					return Result{}, fmt.Errorf("campo inválido")
				}
			}
			next := []Row{}
			for _, r := range rows {
				n := Row{}
				for _, key := range keys {
					k := strings.TrimSpace(key)
					if !field.MatchString(k) {
						return Result{}, fmt.Errorf("campo inválido")
					}
					n[k] = get(r, k)
				}
				next = append(next, n)
			}
			rows = next
		case strings.HasPrefix(stage, "sort "):
			args := strings.Fields(strings.TrimPrefix(stage, "sort "))
			if len(args) < 1 || len(args) > 2 || !field.MatchString(args[0]) || (len(args) == 2 && args[1] != "asc" && args[1] != "desc") {
				return Result{}, fmt.Errorf("use sort campo asc|desc")
			}
			desc := len(args) == 2 && args[1] == "desc"
			sort.SliceStable(rows, func(i, j int) bool {
				c := compare(get(rows[i], args[0]), get(rows[j], args[0]))
				if desc {
					return c > 0
				}
				return c < 0
			})
		case strings.HasPrefix(stage, "limit "):
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(stage, "limit ")))
			if err != nil || n < 1 || n > 1000 {
				return Result{}, fmt.Errorf("limit deve ser 1..1000")
			}
			limit = n
			if len(rows) > limit {
				rows = rows[:limit]
				truncated = true
			}
		case strings.HasPrefix(stage, "summarize "):
			raw := strings.TrimPrefix(stage, "summarize ")
			spec := strings.Split(raw, " by ")
			if len(spec) > 2 {
				return Result{}, fmt.Errorf("summarize inválido")
			}
			group := ""
			if len(spec) == 2 {
				group = strings.TrimSpace(spec[1])
				if !field.MatchString(group) {
					return Result{}, fmt.Errorf("agrupamento inválido")
				}
			}
			expressions := strings.Split(spec[0], ",")
			if len(expressions) > 8 {
				return Result{}, fmt.Errorf("máximo de 8 agregações")
			}
			for _, expr := range expressions {
				m := aggregate.FindStringSubmatch(strings.TrimSpace(expr))
				if m == nil || (m[1] == "count" && m[2] != "") || (m[1] != "count" && !field.MatchString(m[2])) {
					return Result{}, fmt.Errorf("agregação inválida")
				}
			}
			groups := map[string][]Row{}
			if group == "" {
				groups[""] = rows
			} else {
				for _, r := range rows {
					b, _ := json.Marshal(get(r, group))
					k := string(b)
					groups[k] = append(groups[k], r)
				}
			}
			next := []Row{}
			for _, list := range groups {
				n := Row{}
				if group != "" && len(list) > 0 {
					n[group] = get(list[0], group)
				}
				for _, expr := range expressions {
					expr = strings.TrimSpace(expr)
					m := aggregate.FindStringSubmatch(expr)
					if m == nil || (m[1] == "count" && m[2] != "") || (m[1] != "count" && !field.MatchString(m[2])) {
						return Result{}, fmt.Errorf("agregação inválida: %s", expr)
					}
					if m[1] == "count" {
						n[expr] = len(list)
						continue
					}
					count := 0
					sum, min, max := 0.0, 0.0, 0.0
					for _, r := range list {
						v, ok := number(get(r, m[2]))
						if !ok {
							continue
						}
						if count == 0 || v < min {
							min = v
						}
						if count == 0 || v > max {
							max = v
						}
						sum += v
						if math.IsNaN(sum) || math.IsInf(sum, 0) {
							return Result{}, fmt.Errorf("agregação excede faixa numérica")
						}
						count++
					}
					if count == 0 {
						n[expr] = nil
					} else {
						switch m[1] {
						case "sum":
							n[expr] = sum
						case "avg":
							n[expr] = sum / float64(count)
						case "min":
							n[expr] = min
						case "max":
							n[expr] = max
						}
					}
				}
				next = append(next, n)
			}
			rows = next
			sort.Slice(rows, func(i, j int) bool {
				a, _ := json.Marshal(rows[i])
				b, _ := json.Marshal(rows[j])
				return string(a) < string(b)
			})
		default:
			return Result{}, fmt.Errorf("etapa não suportada: %s", stage)
		}
	}
	matched := len(rows)
	if len(rows) > limit {
		rows = rows[:limit]
		truncated = true
	}
	keys := map[string]bool{}
	for _, r := range rows {
		for k := range r {
			keys[k] = true
		}
	}
	columns := []string{}
	for k := range keys {
		columns = append(columns, k)
	}
	sort.Strings(columns)
	return Result{Rows: rows, Columns: columns, Matched: matched, Truncated: truncated, Language: "VQL 1"}, nil
}
