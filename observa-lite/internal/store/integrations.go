package store

import (
	"encoding/json"
	"errors"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var integrationID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)
var credentialPrefix = regexp.MustCompile(`^OBSERVA_[A-Z][A-Z0-9_]{0,70}$`)
var cloudScope = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,150}$`)
var awsRegion = regexp.MustCompile(`^[a-z]{2}(?:-[a-z]+)+-[0-9]+$`)
var profileName = regexp.MustCompile(`^[a-zA-Z0-9_,.-]{1,100}$`)

func ValidateIntegration(v model.Integration) error {
	if !integrationID.MatchString(v.ID) || strings.TrimSpace(v.Name) == "" || len(v.Name) > 200 {
		return errors.New("invalid integration identity")
	}
	switch v.Kind {
	case "aws":
		if !awsRegion.MatchString(v.Region) || strings.HasPrefix(v.Region, "cn-") {
			return errors.New("valid AWS commercial or GovCloud region required")
		}
	case "azure", "gcp":
		if !cloudScope.MatchString(v.Scope) {
			return errors.New("subscription or project required")
		}
	case "snmp_exporter":
		u, e := url.Parse(v.Endpoint)
		if e != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("exporter endpoint must be HTTP(S) without credentials, query or fragment")
		}
		if net.ParseIP(v.Target) == nil {
			return errors.New("network device target must be a literal IPv4 or IPv6 address")
		}
		if !profileName.MatchString(v.Module) || !profileName.MatchString(v.Auth) {
			return errors.New("exporter module and auth profile required")
		}
	default:
		return errors.New("unsupported integration")
	}
	if v.Kind != "snmp_exporter" && !credentialPrefix.MatchString(v.CredentialPrefix) {
		return errors.New("credential prefix must start with OBSERVA_ and contain uppercase letters, numbers and underscores")
	}
	return nil
}
func (s *Store) Integrations() []model.Integration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.Integration{}
	for _, v := range s.data.Integrations {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func (s *Store) Integration(id string) (model.Integration, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Integrations[id]
	return v, ok
}
func (s *Store) PutIntegration(v model.Integration) error {
	if e := ValidateIntegration(v); e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, exists := s.data.Integrations[v.ID]
	if !exists && len(s.data.Integrations) >= 50 {
		return errors.New("max 50 integration profiles")
	}
	previous, hadResult := s.data.IntegrationResults[v.ID]
	s.data.Integrations[v.ID] = v
	comparison := old
	comparison.Enabled = v.Enabled
	comparison.Name = v.Name
	if comparison != v {
		delete(s.data.IntegrationResults, v.ID)
	}
	if e := s.persist(); e != nil {
		if exists {
			s.data.Integrations[v.ID] = old
		} else {
			delete(s.data.Integrations, v.ID)
		}
		if hadResult {
			s.data.IntegrationResults[v.ID] = previous
		}
		return e
	}
	return nil
}
func (s *Store) DeleteIntegration(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.data.Integrations[id]
	result, had := s.data.IntegrationResults[id]
	delete(s.data.Integrations, id)
	delete(s.data.IntegrationResults, id)
	if e := s.persist(); e != nil {
		if ok {
			s.data.Integrations[id] = old
		}
		if had {
			s.data.IntegrationResults[id] = result
		}
		return e
	}
	return nil
}
func (s *Store) SaveIntegrationResult(profile model.Integration, result model.IntegrationResult) error {
	b, e := json.Marshal(result)
	if e != nil {
		return e
	}
	resourceBytes, resourceErr := json.Marshal(result.Resources)
	if resourceErr != nil {
		return resourceErr
	}
	if len(resourceBytes) > 2<<20 || len(result.Resources) > 1000 || len(result.Error) > 2000 {
		return errors.New("integration result exceeds bounds")
	}
	var cloned model.IntegrationResult
	if e = json.Unmarshal(b, &cloned); e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.data.Integrations[profile.ID]; !ok || current != profile {
		return errors.New("integration changed during collection")
	}
	previous, had := s.data.IntegrationResults[profile.ID]
	if had && previous.At.After(result.At) {
		return errors.New("older integration result rejected")
	}
	s.data.IntegrationResults[profile.ID] = cloned
	total := 0
	for _, value := range s.data.IntegrationResults {
		encoded, _ := json.Marshal(value.Resources)
		total += len(encoded)
	}
	if total > 16<<20 {
		if had {
			s.data.IntegrationResults[profile.ID] = previous
		} else {
			delete(s.data.IntegrationResults, profile.ID)
		}
		return errors.New("integration inventory exceeds 16 MiB")
	}
	if e = s.persist(); e != nil {
		if had {
			s.data.IntegrationResults[profile.ID] = previous
		} else {
			delete(s.data.IntegrationResults, profile.ID)
		}
		return e
	}
	return nil
}
func (s *Store) IntegrationResults() map[string]model.IntegrationResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, _ := json.Marshal(s.data.IntegrationResults)
	out := map[string]model.IntegrationResult{}
	_ = json.Unmarshal(b, &out)
	return out
}
