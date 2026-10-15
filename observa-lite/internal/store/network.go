package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"sort"
	"strings"
)

func SealSNMPCredentials(key, id string, credentials model.SNMPCredentials) (string, error) {
	sum := sha256.Sum256([]byte("vetra-snmp-v1:" + key))
	block, e := aes.NewCipher(sum[:])
	if e != nil {
		return "", e
	}
	a, e := cipher.NewGCM(block)
	if e != nil {
		return "", e
	}
	n := make([]byte, a.NonceSize())
	if _, e = rand.Read(n); e != nil {
		return "", e
	}
	b, e := json.Marshal(credentials)
	if e != nil {
		return "", e
	}
	return base64.StdEncoding.EncodeToString(a.Seal(n, n, b, []byte(id))), nil
}
func (s *Store) HasSNMPCredentials(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.IntegrationSecrets[id] != ""
}
func (s *Store) SNMPCredentials(key, id string) (model.SNMPCredentials, error) {
	s.mu.RLock()
	encoded := s.data.IntegrationSecrets[id]
	s.mu.RUnlock()
	var out model.SNMPCredentials
	b, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil {
		return out, errors.New("credenciais SNMP indisponíveis")
	}
	sum := sha256.Sum256([]byte("vetra-snmp-v1:" + key))
	block, _ := aes.NewCipher(sum[:])
	a, _ := cipher.NewGCM(block)
	if len(b) < a.NonceSize() {
		return out, errors.New("credenciais SNMP não configuradas")
	}
	plain, e := a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], []byte(id))
	if e != nil {
		return out, errors.New("não foi possível abrir as credenciais; recadastre após trocar a chave administrativa")
	}
	e = json.Unmarshal(plain, &out)
	return out, e
}
func (s *Store) NetworkTemplates() []model.NetworkTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, _ := json.Marshal(s.data.NetworkTemplates)
	m := map[string]model.NetworkTemplate{}
	_ = json.Unmarshal(b, &m)
	out := []model.NetworkTemplate{}
	for _, t := range m {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func (s *Store) NetworkTemplate(id string) (model.NetworkTemplate, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.data.NetworkTemplates[id]
	if !ok {
		return value, false
	}
	b, _ := json.Marshal(value)
	var out model.NetworkTemplate
	_ = json.Unmarshal(b, &out)
	return out, true
}
func (s *Store) DeleteNetworkTemplate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "generic-snmp" {
		return errors.New("perfil nativo não pode ser excluído")
	}
	for _, p := range s.data.Integrations {
		if p.TemplateID == id {
			return errors.New("template está vinculado a um equipamento")
		}
	}
	old, ok := s.data.NetworkTemplates[id]
	delete(s.data.NetworkTemplates, id)
	if e := s.persist(); e != nil {
		if ok {
			s.data.NetworkTemplates[id] = old
		}
		return e
	}
	return nil
}
func (s *Store) PutNetworkTemplates(templates []model.NetworkTemplate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.NetworkTemplates == nil {
		s.data.NetworkTemplates = map[string]model.NetworkTemplate{}
	}
	old := map[string]model.NetworkTemplate{}
	for id, t := range s.data.NetworkTemplates {
		old[id] = t
	}
	for _, t := range templates {
		if !integrationID.MatchString(t.ID) || len(t.Name) > 256 || len(t.Items) > 2000 || len(t.Discovery) > 100 {
			s.data.NetworkTemplates = old
			return errors.New("template excede os limites")
		}
		s.data.NetworkTemplates[t.ID] = t
	}
	if len(s.data.NetworkTemplates) > 100 {
		s.data.NetworkTemplates = old
		return errors.New("limite de 100 templates")
	}
	b, _ := json.Marshal(s.data.NetworkTemplates)
	if len(b) > 8<<20 {
		s.data.NetworkTemplates = old
		return errors.New("catálogo excede 8 MiB")
	}
	if e := s.persist(); e != nil {
		s.data.NetworkTemplates = old
		return e
	}
	return nil
}

// Network metric history uses the same bounded native signal store as OTLP.
func networkMetricBatch(profile model.Integration, result model.IntegrationResult) []model.MetricPoint {
	out := []model.MetricPoint{}
	for _, resource := range result.Resources {
		add := func(key string, value float64, index string) {
			if len(out) >= 4096 {
				return
			}
			kind := "gauge"
			monotonic := false
			if strings.HasPrefix(key, "ifHC") || strings.HasSuffix(key, "Octets") || strings.HasSuffix(key, "Errors") || strings.HasSuffix(key, "Discards") || strings.HasSuffix(key, ".raw_counter") {
				kind = "sum"
				monotonic = true
			}
			out = append(out, model.MetricPoint{Name: "network.snmp." + key, Kind: kind, Monotonic: monotonic, Temporality: "cumulative", Timestamp: result.At, ServiceName: profile.Name, ResourceAttributes: map[string]any{"device.id": profile.ID, "device.address": profile.Target, "host.group": profile.HostGroup, "template.id": profile.TemplateID}, Attributes: map[string]any{"interface.index": index}, Data: map[string]any{"asDouble": value}})
		}
		keys := []string{}
		for k := range resource.Metrics {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			add(k, resource.Metrics[k], "")
		}
		for _, iface := range resource.Interfaces {
			keys = []string{}
			for k := range iface.Metrics {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				add(k, iface.Metrics[k], iface.Index)
			}
		}
	}
	return out
}
