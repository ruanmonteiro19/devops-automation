package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type componentDownload struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Bytes    int64  `json:"bytes"`
	SHA256   string `json:"sha256"`
	URL      string `json:"url"`
}

var artifactName = regexp.MustCompile(`^(vetra-agent|vetra-relay|vetra-relay-web)-(linux|windows)-(amd64|arm64)(\.exe)?$`)

func downloadDir() string {
	if v := os.Getenv("VETRA_DOWNLOAD_DIR"); v != "" {
		return v
	}
	return "./downloads"
}
func (s *Server) downloadRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/downloads", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		entries, e := os.ReadDir(downloadDir())
		out := []componentDownload{}
		if e != nil && !os.IsNotExist(e) {
			fail(w, 500, fmt.Errorf("não foi possível ler os pacotes"))
			return
		}
		for _, entry := range entries {
			if !artifactName.MatchString(entry.Name()) || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			f, e := os.Open(filepath.Join(downloadDir(), entry.Name()))
			if e != nil {
				continue
			}
			info, e := f.Stat()
			if e != nil || !info.Mode().IsRegular() {
				f.Close()
				continue
			}
			h := sha256.New()
			_, e = io.Copy(h, f)
			f.Close()
			if e != nil {
				continue
			}
			name := "Vetra Agent"
			if strings.HasPrefix(entry.Name(), "vetra-relay-web-") {
				name = "Vetra Relay Web"
			} else if strings.HasPrefix(entry.Name(), "vetra-relay-") {
				name = "Vetra Relay"
			}
			parts := artifactName.FindStringSubmatch(entry.Name())
			out = append(out, componentDownload{ID: entry.Name(), Name: name, Platform: parts[2] + " / " + parts[3], Bytes: info.Size(), SHA256: hex.EncodeToString(h.Sum(nil)), URL: "/api/v1/downloads/" + entry.Name()})
		}
		write(w, 200, map[string]any{"components": out, "storage": s.Store.StorageInfo()})
	}))
	m.HandleFunc("GET /api/v1/downloads/{name}", s.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !artifactName.MatchString(name) {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(downloadDir(), name)
		info, e := os.Lstat(path)
		if e != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		f, e := os.Open(path)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		w.Header().Set("Cache-Control", "no-store")
		http.ServeContent(w, r, name, info.ModTime(), f)
	}))
}
