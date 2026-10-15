package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/collectors"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

func (s *Server) integrationList(w http.ResponseWriter, r *http.Request) {
	collecting := []string{}
	s.mu.Lock()
	for key := range s.running {
		if strings.HasPrefix(key, "integration:") {
			collecting = append(collecting, strings.TrimPrefix(key, "integration:"))
		}
	}
	s.mu.Unlock()
	sort.Strings(collecting)
	write(w, 200, map[string]any{"integrations": s.Store.Integrations(), "results": s.Store.IntegrationResults(), "collecting": collecting})
}
func (s *Server) integrationSave(w http.ResponseWriter, r *http.Request) {
	var request struct {
		model.Integration
		Credentials *model.SNMPCredentials `json:"credentials,omitempty"`
	}
	if !decode(w, r, &request) {
		return
	}
	p := request.Integration
	secret, err := s.prepareSNMPProfile(p, request.Credentials)
	if err != nil {
		fail(w, 400, err)
		return
	}
	if p.Kind == "snmp" {
		if old, ok := s.Store.Integration(p.ID); ok {
			p.SNMPRevision = old.SNMPRevision
		}
		if secret != "" {
			sum := sha256.Sum256([]byte(secret))
			p.SNMPRevision = hex.EncodeToString(sum[:8])
		}
	}
	if e := store.ValidateIntegration(p); e != nil {
		fail(w, 400, e)
		return
	}
	if e := s.Store.PutIntegrationWithSecret(p, secret); e != nil {
		fail(w, 500, e)
		return
	}
	write(w, 200, map[string]bool{"saved": true})
}
func (s *Server) integrationDelete(w http.ResponseWriter, r *http.Request) {
	if e := s.Store.DeleteIntegration(r.PathValue("id")); e != nil {
		fail(w, 500, e)
		return
	}
	write(w, 200, map[string]bool{"deleted": true})
}
func (s *Server) reserveProfile(p model.Integration) (func(), error) {
	key := "integration:" + p.ID
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[key] || len(s.running) >= 4 {
		return nil, errIntegrationBusy
	}
	s.running[key] = true
	return func() { s.mu.Lock(); delete(s.running, key); s.mu.Unlock() }, nil
}
func (s *Server) collectProfile(ctx context.Context, p model.Integration) (model.IntegrationResult, error) {
	release, e := s.reserveProfile(p)
	if e != nil {
		return model.IntegrationResult{}, e
	}
	defer release()
	return s.runProfile(ctx, p)
}
func (s *Server) runProfile(ctx context.Context, p model.Integration) (model.IntegrationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	result, e := s.Runner.CollectIntegration(ctx, p)
	if ctx.Err() == context.Canceled {
		return result, ctx.Err()
	}
	if e != nil {
		if old, ok := s.Store.IntegrationResults()[p.ID]; ok {
			result.Resources = old.Resources
			result.SuccessAt = old.SuccessAt
		}
	} else {
		result.SuccessAt = result.At
		if p.Kind == "snmp_exporter" || p.Kind == "snmp" {
			if previous, ok := s.Store.IntegrationResults()[p.ID]; ok {
				collectors.DeviceRates(&result, previous)
			}
		}
	}
	if saveErr := s.Store.SaveIntegrationResult(p, result); saveErr != nil {
		failure := model.IntegrationResult{At: result.At, Status: "error", Error: fmt.Sprintf("inventory not updated: %v", saveErr), Resources: []model.IntegrationResource{}}
		if previous, ok := s.Store.IntegrationResults()[p.ID]; ok {
			failure.Resources = previous.Resources
			failure.SuccessAt = previous.SuccessAt
		}
		if fallbackErr := s.Store.SaveIntegrationResult(p, failure); fallbackErr == nil {
			return failure, nil
		}
		log.Printf("integration inventory persistence failed for profile %s", p.ID)
		return result, saveErr
	}
	return result, nil
}

var errIntegrationBusy = integrationError("collection already running or worker limit reached")

type integrationError string

func (e integrationError) Error() string { return string(e) }
func (s *Server) integrationCollect(w http.ResponseWriter, r *http.Request) {
	p, ok := s.Store.Integration(r.PathValue("id"))
	if !ok {
		http.Error(w, "integration not found", 404)
		return
	}
	if !p.Enabled {
		http.Error(w, "integration disabled", 409)
		return
	}
	release, e := s.reserveProfile(p)
	if e != nil {
		fail(w, 409, e)
		return
	}
	go func() { defer release(); _, _ = s.runProfile(context.WithoutCancel(r.Context()), p) }()
	write(w, 202, map[string]bool{"accepted": true})
}
func (s *Server) pollIntegrations(ctx context.Context) {
	results := s.Store.IntegrationResults()
	for _, p := range s.Store.Integrations() {
		if !p.Enabled {
			continue
		}
		interval := 5 * time.Minute
		if p.Kind == "snmp" && p.IntervalSeconds >= 30 {
			interval = time.Duration(p.IntervalSeconds) * time.Second
		}
		if previous, ok := results[p.ID]; ok && time.Since(previous.At) < interval {
			continue
		}
		p := p
		go func() { _, _ = s.collectProfile(ctx, p) }()
	}
}
