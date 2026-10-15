package store

import (
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIntegrationPersistenceInvalidationAndDeletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, _ := New(path)
	p := model.Integration{ID: "aws", Name: "Production", Kind: "aws", Region: "us-east-1", CredentialPrefix: "OBSERVA_AWS", Enabled: true}
	if e := s.PutIntegration(p); e != nil {
		t.Fatal(e)
	}
	result := model.IntegrationResult{At: time.Now(), SuccessAt: time.Now(), Status: "ok", Resources: []model.IntegrationResource{{ID: "i-1", Name: "api", Kind: "aws.ec2.instance"}}}
	if e := s.SaveIntegrationResult(p, result); e != nil {
		t.Fatal(e)
	}
	result.Resources[0].Name = "mutation"
	reopened, e := New(path)
	if e != nil {
		t.Fatal(e)
	}
	if reopened.IntegrationResults()[p.ID].Resources[0].Name != "api" {
		t.Fatal("result mutated")
	}
	p.Enabled = false
	if e = s.PutIntegration(p); e != nil {
		t.Fatal(e)
	}
	if len(s.IntegrationResults()[p.ID].Resources) != 1 {
		t.Fatal("pause removed last inventory")
	}
	p.Region = "eu-west-1"
	if e = s.PutIntegration(p); e != nil {
		t.Fatal(e)
	}
	if _, ok := s.IntegrationResults()[p.ID]; ok {
		t.Fatal("old scope inventory retained")
	}
	pOld := p
	pOld.Region = "us-east-1"
	if e = s.SaveIntegrationResult(pOld, result); e == nil {
		t.Fatal("inflight old configuration accepted")
	}
	if e = s.DeleteIntegration(p.ID); e != nil {
		t.Fatal(e)
	}
	if _, ok := s.Integration(p.ID); ok {
		t.Fatal("deletion failed")
	}
}
func TestIntegrationValidationAndRollback(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "state.json"))
	p := model.Integration{ID: "switch", Name: "Core", Kind: "snmp_exporter", Endpoint: "http://exporter:9116", Target: "10.0.0.1", Module: "if_mib", Auth: "switch_v3"}
	if e := s.PutIntegration(p); e != nil {
		t.Fatal(e)
	}
	for _, target := range []string{"example.com", "127.0.0.1:161", "bad"} {
		invalid := p
		invalid.Target = target
		if e := s.PutIntegration(invalid); e == nil {
			t.Fatal("invalid target accepted")
		}
	}
	invalid := p
	invalid.Endpoint = "http://user:secret@host"
	if e := s.PutIntegration(invalid); e == nil {
		t.Fatal("embedded secret accepted")
	}
	block := filepath.Join(t.TempDir(), "block")
	if e := os.WriteFile(block, []byte("x"), 0600); e != nil {
		t.Fatal(e)
	}
	s.file = filepath.Join(block, "state.json")
	p.Name = "Changed"
	if e := s.PutIntegration(p); e == nil {
		t.Fatal("expected disk error")
	}
	got, _ := s.Integration(p.ID)
	if got.Name != "Core" {
		t.Fatal("failed write mutated config")
	}
	if e := s.DeleteIntegration(p.ID); e == nil {
		t.Fatal("expected delete disk error")
	}
	if _, ok := s.Integration(p.ID); !ok {
		t.Fatal("failed deletion mutated state")
	}
}
