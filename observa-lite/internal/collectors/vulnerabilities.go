package collectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net/http"
	"net/url"
	"strings"
)

type osvBrief struct {
	ID       string `json:"id"`
	Modified string `json:"modified"`
}
type VulnerabilityFinding struct {
	ID        string   `json:"id"`
	Package   string   `json:"package"`
	Version   string   `json:"version"`
	Ecosystem string   `json:"ecosystem"`
	Severity  string   `json:"severity"`
	Summary   string   `json:"summary"`
	Fixed     []string `json:"fixed"`
	URL       string   `json:"url"`
	Modified  string   `json:"modified"`
}

func (r Runner) ScanPackages(ctx context.Context, packages []model.SoftwarePackage) (map[string]any, error) {
	if len(packages) < 1 || len(packages) > 500 {
		return nil, fmt.Errorf("informe 1..500 pacotes com versão")
	}
	findings := []VulnerabilityFinding{}
	complete := true
	detailsComplete := true
	for start := 0; start < len(packages); start += 50 {
		end := start + 50
		if end > len(packages) {
			end = len(packages)
		}
		pending := []int{}
		tokens := map[int]string{}
		for i := start; i < end; i++ {
			pending = append(pending, i)
		}
		for page := 0; len(pending) > 0; page++ {
			if page >= 5 {
				complete = false
				break
			}
			queries := []any{}
			for _, i := range pending {
				p := packages[i]
				q := map[string]any{"version": p.Version, "package": map[string]string{"name": p.Name, "ecosystem": p.Ecosystem}}
				if tokens[i] != "" {
					q["page_token"] = tokens[i]
				}
				queries = append(queries, q)
			}
			payload, _ := json.Marshal(map[string]any{"queries": queries})
			b, e := r.integrationRequest(ctx, "POST", "https://api.osv.dev/v1/querybatch", bytes.NewReader(payload), http.Header{"Content-Type": []string{"application/json"}})
			if e != nil {
				return nil, fmt.Errorf("consulta OSV falhou; nenhum resultado limpo pode ser inferido: %w", e)
			}
			var reply struct {
				Results []struct {
					Vulns []osvBrief `json:"vulns"`
					Next  string     `json:"next_page_token"`
				} `json:"results"`
			}
			if json.Unmarshal(b, &reply) != nil || len(reply.Results) != len(pending) {
				return nil, fmt.Errorf("resposta OSV inválida")
			}
			next := []int{}
			for k, v := range reply.Results {
				i := pending[k]
				p := packages[i]
				if v.Next != "" {
					tokens[i] = v.Next
					next = append(next, i)
				}
				for _, vuln := range v.Vulns {
					if len(findings) >= 500 {
						complete = false
						continue
					}
					findings = append(findings, VulnerabilityFinding{ID: vuln.ID, Package: p.Name, Version: p.Version, Ecosystem: p.Ecosystem, Severity: "Unknown", Fixed: []string{}, URL: "https://osv.dev/vulnerability/" + url.PathEscape(vuln.ID), Modified: vuln.Modified})
				}
			}
			pending = next
		}
	}
	details := map[string]map[string]any{}
	for i := range findings {
		f := &findings[i]
		d, ok := details[f.ID]
		if !ok {
			if len(details) >= 100 {
				detailsComplete = false
				continue
			}
			b, e := r.integrationRequest(ctx, "GET", "https://api.osv.dev/v1/vulns/"+url.PathEscape(f.ID), nil, http.Header{})
			if e != nil {
				detailsComplete = false
				details[f.ID] = nil
				continue
			}
			if json.Unmarshal(b, &d) != nil {
				detailsComplete = false
				continue
			}
			details[f.ID] = d
		}
		if d == nil {
			detailsComplete = false
			continue
		}
		if withdrawn, ok := d["withdrawn"].(string); ok && withdrawn != "" {
			f.Severity = "Withdrawn"
		}
		if summary, ok := d["summary"].(string); ok {
			if len(summary) > 500 {
				summary = summary[:500]
			}
			f.Summary = summary
		}
		if db, ok := d["database_specific"].(map[string]any); ok && f.Severity != "Withdrawn" {
			if v, ok := db["severity"].(string); ok {
				switch strings.ToUpper(v) {
				case "CRITICAL":
					f.Severity = "Critical"
				case "HIGH":
					f.Severity = "High"
				case "MODERATE", "MEDIUM":
					f.Severity = "Medium"
				case "LOW":
					f.Severity = "Low"
				}
			}
		}
		affected, _ := d["affected"].([]any)
		seen := map[string]bool{}
		for _, a := range affected {
			item, _ := a.(map[string]any)
			pkg, _ := item["package"].(map[string]any)
			if pkg["name"] != f.Package || pkg["ecosystem"] != f.Ecosystem {
				continue
			}
			ranges, _ := item["ranges"].([]any)
			for _, r := range ranges {
				rr, _ := r.(map[string]any)
				events, _ := rr["events"].([]any)
				for _, event := range events {
					ev, _ := event.(map[string]any)
					if fix, ok := ev["fixed"].(string); ok && !seen[fix] {
						f.Fixed = append(f.Fixed, fix)
						seen[fix] = true
					}
				}
			}
		}
	}
	return map[string]any{"findings": findings, "packages_scanned": len(packages), "matches": len(findings), "query_complete": complete, "details_complete": detailsComplete, "source": "OSV.dev", "assessment": "package-version-match; runtime exploitability not assessed"}, nil
}
