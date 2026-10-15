package server

import (
	"errors"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/network"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http"
)

func (s *Server) networkTemplateList(w http.ResponseWriter, r *http.Request) {
	templates := append([]model.NetworkTemplate{network.Builtin()}, s.Store.NetworkTemplates()...)
	configured := map[string]bool{}
	for _, p := range s.Store.Integrations() {
		if p.Kind == "snmp" {
			configured[p.ID] = s.Store.HasSNMPCredentials(p.ID)
		}
	}
	write(w, 200, map[string]any{"templates": templates, "credentials_configured": configured, "collector": "server", "capabilities": map[string]any{"protocols": []string{"SNMPv1", "SNMPv2c", "SNMPv3"}, "template_formats": []string{"YAML", "JSON", "XML"}, "netflow": false, "ipfix": false, "snmp_traps": false, "relay_collection": false}})
}
func (s *Server) networkTemplateImport(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Document string `json:"document"`
		Preview  bool   `json:"preview"`
	}
	if !decode(w, r, &request) {
		return
	}
	templates, e := network.ParseTemplates([]byte(request.Document))
	if e != nil {
		fail(w, 400, e)
		return
	}
	if !request.Preview {
		if e = s.Store.PutNetworkTemplates(templates); e != nil {
			fail(w, 400, e)
			return
		}
	}
	write(w, 200, map[string]any{"templates": templates, "saved": !request.Preview})
}
func (s *Server) prepareSNMPProfile(p model.Integration, c *model.SNMPCredentials) (string, error) {
	if p.Kind != "snmp" {
		if c != nil {
			return "", errors.New("credenciais diretas são exclusivas de SNMP nativo")
		}
		return "", nil
	}
	if p.TemplateID != "generic-snmp" {
		if _, ok := s.Store.NetworkTemplate(p.TemplateID); !ok {
			return "", errors.New("selecione um template existente")
		}
	}
	if c == nil {
		old, e := s.Store.SNMPCredentials(s.AdminToken, p.ID)
		if e != nil {
			return "", e
		}
		if e = network.ValidateCredentials(p.SNMPVersion, old); e != nil {
			return "", e
		}
		return "", nil
	}
	if e := network.ValidateCredentials(p.SNMPVersion, *c); e != nil {
		return "", e
	}
	return store.SealSNMPCredentials(s.AdminToken, p.ID, *c)
}

func (s *Server) networkTemplateDelete(w http.ResponseWriter, r *http.Request) {
	if e := s.Store.DeleteNetworkTemplate(r.PathValue("id")); e != nil {
		fail(w, 409, e)
		return
	}
	write(w, 200, map[string]bool{"deleted": true})
}
