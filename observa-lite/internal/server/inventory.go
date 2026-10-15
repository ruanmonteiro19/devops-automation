package server

import (
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http"
)

func (s *Server) editInventory(w http.ResponseWriter, r *http.Request) {
	var e store.InventoryEdit
	if !decode(w, r, &e) {
		return
	}
	if err := s.Store.EditInventory(r.PathValue("id"), e); err != nil {
		fail(w, 400, err)
		return
	}
	write(w, 200, map[string]bool{"saved": true})
}
