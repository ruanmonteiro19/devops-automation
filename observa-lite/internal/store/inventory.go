package store

import (
	"encoding/json"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"strings"
)

type InventorySettings struct {
	Name      string            `json:"name"`
	Group     string            `json:"group"`
	GroupSet  bool              `json:"group_set"`
	Processes map[string]string `json:"processes,omitempty"`
}
type InventoryEdit struct {
	OriginalName    *string `json:"original_name"`
	OriginalService *string `json:"original_service"`
	Name            *string `json:"name"`
	Group           *string `json:"host_group"`
	PID             *int    `json:"pid"`
	ProcessName     *string `json:"process_name"`
	Reset           bool    `json:"reset"`
}

func processKey(p model.Process) string {
	b, _ := json.Marshal([]string{p.Name, p.Service})
	return string(b)
}
func (s *Store) projectInventory(h model.Host) model.Host {
	v := s.data.InventorySettings[h.ID]
	if v.Name != "" {
		h.Name = v.Name
	}
	if v.GroupSet {
		h.HostGroup = v.Group
	}
	h.Processes = append([]model.Process{}, h.Processes...)
	for i, p := range h.Processes {
		h.Processes[i].DisplayName = v.Processes[processKey(p)]
	}
	return h
}
func (s *Store) EditInventory(id string, e InventoryEdit) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.data.Hosts[id]
	if !ok {
		return fmt.Errorf("host não encontrado")
	}
	old := s.data.InventorySettings
	next := make(map[string]InventorySettings, len(old)+1)
	for k, v := range old {
		next[k] = v
	}
	v := next[id]
	aliases := map[string]string{}
	for k, a := range v.Processes {
		aliases[k] = a
	}
	v.Processes = aliases
	clean := func(p *string, max int) (string, error) {
		if p == nil {
			return "", nil
		}
		s := strings.TrimSpace(*p)
		if len(s) > max || strings.ContainsAny(s, "\r\n\x00") {
			return "", fmt.Errorf("nome inválido")
		}
		return s, nil
	}
	name, err := clean(e.Name, 128)
	if err != nil {
		return err
	}
	group, err := clean(e.Group, 100)
	if err != nil {
		return err
	}
	alias, err := clean(e.ProcessName, 128)
	if err != nil {
		return err
	}
	if e.Reset {
		v.Name = ""
		v.Group = ""
		v.GroupSet = false
		next[id] = v
	} else {
		if e.Name != nil {
			v.Name = name
		}
		if e.Group != nil {
			v.Group = group
			v.GroupSet = true
		}
		if e.ProcessName != nil {
			if e.PID == nil {
				return fmt.Errorf("PID obrigatório")
			}
			found := false
			for _, p := range h.Processes {
				if p.PID == *e.PID {
					if e.OriginalName == nil || e.OriginalService == nil || *e.OriginalName != p.Name || *e.OriginalService != p.Service {
						return fmt.Errorf("o processo mudou desde a seleção; atualize o inventário")
					}
					if alias == "" {
						delete(v.Processes, processKey(p))
					} else {
						v.Processes[processKey(p)] = alias
					}
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("processo não encontrado")
			}
		}
		next[id] = v
	}
	s.data.InventorySettings = next
	if err := s.persist(); err != nil {
		s.data.InventorySettings = old
		return err
	}
	return nil
}
