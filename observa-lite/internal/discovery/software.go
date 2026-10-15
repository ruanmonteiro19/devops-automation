package discovery

import (
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"io"
	"os"
	"sort"
	"strings"
)

// Inspect build metadata/manifests, never execute binaries or scan the filesystem recursively.
func DiscoverSoftware(d model.Discovery) []model.SoftwarePackage {
	out := []model.SoftwarePackage{}
	seen := map[string]bool{}
	add := func(p model.SoftwarePackage) {
		if p.Name == "" || p.Version == "" || p.Version == "(devel)" || len(p.Name) > 256 || len(p.Version) > 128 || len(out) >= 500 {
			return
		}
		k := p.Ecosystem + ":" + p.Name + ":" + p.Version
		if !seen[k] {
			seen[k] = true
			out = append(out, p)
		}
	}
	attempts := 0
	processes := append([]model.Process{}, d.Processes...)
	sort.SliceStable(processes, func(i, j int) bool {
		rank := func(p model.Process) int {
			if p.Runtime == "nodejs" {
				return 0
			}
			if p.Runtime == "go" {
				return 1
			}
			return 2
		}
		return rank(processes[i]) < rank(processes[j])
	})
	for _, p := range processes {
		if attempts >= 10 {
			break
		}
		if p.Runtime == "go" || (p.Runtime == "process" && !model.IsSystemProcess(p)) {
			attempts++
			info, e := buildinfo.ReadFile(fmt.Sprintf("/proc/%d/exe", p.PID))
			if e != nil {
				continue
			}
			for _, mod := range info.Deps {
				if mod.Replace != nil {
					mod = mod.Replace
				}
				add(model.SoftwarePackage{Name: mod.Path, Version: mod.Version, Ecosystem: "Go", Source: "go-build-metadata"})
			}
		}
		if p.Runtime == "nodejs" {
			attempts++
			f, e := os.Open(fmt.Sprintf("/proc/%d/cwd/package-lock.json", p.PID))
			if e != nil {
				continue
			}
			var lock struct {
				Packages map[string]struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Dev     bool   `json:"dev"`
				} `json:"packages"`
			}
			e = json.NewDecoder(io.LimitReader(f, 4<<20)).Decode(&lock)
			_ = f.Close()
			if e != nil {
				continue
			}
			for path, mod := range lock.Packages {
				if mod.Dev || !strings.Contains(path, "node_modules/") {
					continue
				}
				name := mod.Name
				if name == "" {
					name = path[strings.LastIndex(path, "node_modules/")+len("node_modules/"):]
				}
				add(model.SoftwarePackage{Name: name, Version: mod.Version, Ecosystem: "npm", Source: "package-lock"})
			}
		}
	}
	return out
}
