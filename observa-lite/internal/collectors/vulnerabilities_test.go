package collectors

import (
	"context"
	"encoding/json"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net/http"
	"testing"
)

func TestOSVPaginationDetailsAndCoverage(t *testing.T) {
	pages := 0
	runner := mockProvider(func(r *http.Request) string {
		if r.URL.Path == "/v1/querybatch" {
			pages++
			var q struct {
				Queries []map[string]any `json:"queries"`
			}
			if e := json.NewDecoder(r.Body).Decode(&q); e != nil {
				t.Fatal(e)
			}
			if pages == 1 {
				return `{"results":[{"vulns":[{"id":"GHSA-test-1"}],"next_page_token":"next"}]}`
			}
			if q.Queries[0]["page_token"] != "next" {
				t.Fatal("missing page token")
			}
			return `{"results":[{"vulns":[{"id":"GHSA-test-2"}]}]}`
		}
		return `{"summary":"Example","database_specific":{"severity":"HIGH"},"affected":[{"package":{"name":"example","ecosystem":"npm"},"ranges":[{"events":[{"fixed":"2.0.0"}]}]}]}`
	})
	result, e := runner.ScanPackages(context.Background(), []model.SoftwarePackage{{Name: "example", Version: "1.0.0", Ecosystem: "npm"}})
	if e != nil || pages != 2 || result["matches"] != 2 || result["query_complete"] != true {
		t.Fatalf("%+v %v", result, e)
	}
	f := result["findings"].([]VulnerabilityFinding)
	if f[0].Severity != "High" || len(f[0].Fixed) != 1 || f[0].Fixed[0] != "2.0.0" {
		t.Fatalf("%+v", f)
	}
	runner = mockProvider(func(r *http.Request) string {
		if r.Method == "POST" {
			return `{"results":[{"next_page_token":"forever"}]}`
		}
		return `{}`
	})
	result, e = runner.ScanPackages(context.Background(), []model.SoftwarePackage{{Name: "example", Version: "1", Ecosystem: "npm"}})
	if e != nil || result["query_complete"] != false {
		t.Fatalf("pagination cap %+v %v", result, e)
	}
	runner = mockProvider(func(r *http.Request) string { return `{"results":[]}` })
	if _, e = runner.ScanPackages(context.Background(), []model.SoftwarePackage{{Name: "example", Version: "1", Ecosystem: "npm"}}); e == nil {
		t.Fatal("malformed response presented as clean")
	}
}
func TestDatabaseMissingCredentialsDoNotLeak(t *testing.T) {
	_, e := (Runner{}).CheckDatabase(context.Background(), model.WorkspaceObject{Engine: "postgresql", CredentialEnv: "OBSERVA_DB_MISSING_DSN"})
	if e == nil {
		t.Fatal("missing credential accepted")
	}
}
