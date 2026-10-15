package server

import (
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net/http"
	"net/url"
	"strings"
)

func ParseSoftwareDocument(raw json.RawMessage) ([]model.SoftwarePackage, error) {
	if len(raw) > 4<<20 {
		return nil, fmt.Errorf("documento excede 4 MiB")
	}
	var doc struct {
		BOMFormat  string `json:"bomFormat"`
		Components []struct {
			Name    string `json:"name"`
			Group   string `json:"group"`
			Version string `json:"version"`
			PURL    string `json:"purl"`
		} `json:"components"`
		Packages json.RawMessage `json:"packages"`
	}
	if e := json.Unmarshal(raw, &doc); e != nil {
		return nil, e
	}
	out := []model.SoftwarePackage{}
	seen := map[string]bool{}
	add := func(p model.SoftwarePackage) {
		k := p.Ecosystem + ":" + p.Name + ":" + p.Version
		if !seen[k] && p.Name != "" && p.Version != "" {
			out = append(out, p)
			seen[k] = true
		}
	}
	if strings.EqualFold(doc.BOMFormat, "CycloneDX") {
		ecosystems := map[string]string{"npm": "npm", "pypi": "PyPI", "golang": "Go", "maven": "Maven", "nuget": "NuGet", "composer": "Packagist", "gem": "RubyGems", "cargo": "crates.io"}
		for _, c := range doc.Components {
			if !strings.HasPrefix(c.PURL, "pkg:") {
				continue
			}
			purl := strings.SplitN(strings.TrimPrefix(c.PURL, "pkg:"), "/", 2)
			if len(purl) != 2 {
				continue
			}
			ecosystem := ecosystems[purl[0]]
			if ecosystem == "" {
				continue
			}
			path := strings.SplitN(purl[1], "?", 2)[0]
			at := strings.LastIndex(path, "@")
			version := c.Version
			if at > 0 {
				if version == "" {
					version = path[at+1:]
				}
				path = path[:at]
			}
			name, _ := url.PathUnescape(path)
			if ecosystem == "Maven" {
				name = strings.Replace(name, "/", ":", 1)
			}
			add(model.SoftwarePackage{Name: name, Version: version, Ecosystem: ecosystem, Source: "CycloneDX"})
		}
	}
	if len(out) == 0 && len(doc.Packages) > 0 {
		var packages map[string]struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Dev     bool   `json:"dev"`
		}
		if json.Unmarshal(doc.Packages, &packages) == nil {
			for path, p := range packages {
				if p.Dev || !strings.Contains(path, "node_modules/") {
					continue
				}
				name := p.Name
				if name == "" {
					name = path[strings.LastIndex(path, "node_modules/")+len("node_modules/"):]
				}
				add(model.SoftwarePackage{Name: name, Version: p.Version, Ecosystem: "npm", Source: "package-lock"})
			}
		}
	}
	if len(out) < 1 || len(out) > 500 {
		return nil, fmt.Errorf("informe CycloneDX com purls ou package-lock v2/v3, com 1..500 pacotes de produção suportados")
	}
	return out, nil
}
func (s *Server) importSoftware(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Name     string          `json:"name"`
		Document json.RawMessage `json:"document"`
	}
	if !decode(w, r, &p) {
		return
	}
	packages, e := ParseSoftwareDocument(p.Document)
	if e != nil {
		fail(w, 400, e)
		return
	}
	o := model.WorkspaceObject{ID: "software-" + operationID(), Name: p.Name, Kind: "software", Packages: packages}
	if e := s.Store.PutWorkspace(o); e != nil {
		fail(w, 400, e)
		return
	}
	write(w, 200, o)
}
