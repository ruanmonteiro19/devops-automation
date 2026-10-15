package server

import (
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http"
	"strings"
)

func (s *Server) latestData(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	host, group, q, source, status := strings.ToLower(query.Get("host")), strings.ToLower(query.Get("group")), strings.ToLower(query.Get("q")), query.Get("source"), query.Get("status")
	if len(host) > 512 || len(group) > 128 || len(q) > 512 {
		http.Error(w, "filtro excede o limite", 400)
		return
	}
	filtered := s.Store.LatestDataFiltered(func(row store.LatestItem) bool {
		if host != "" && !strings.Contains(strings.ToLower(row.Host+" "+row.HostID), host) || group != "" && !strings.Contains(strings.ToLower(row.Group), group) || q != "" && !strings.Contains(strings.ToLower(row.Name+" "+row.Key), q) || source != "" && row.Source != source || status != "" && row.Status != status {
			return false
		}
		if row.Status == "Não suportado" && query.Get("unsupported") != "true" && status != "Não suportado" {
			return false
		}
		return true
	})
	write(w, 200, map[string]any{"items": filtered, "total": len(filtered), "retention_hours": 24, "series_limit": 5000, "truncated": len(filtered) >= 5000})
}
func (s *Server) latestHistory(w http.ResponseWriter, r *http.Request) {
	from, to, e := apmWindow(r)
	if e != nil {
		fail(w, 400, e)
		return
	}
	id := r.PathValue("id")
	if len(id) != 32 {
		http.Error(w, "ID de série inválido", 400)
		return
	}
	write(w, 200, map[string]any{"items": s.Store.LatestHistory(id, from, to), "from": from, "to": to})
}
