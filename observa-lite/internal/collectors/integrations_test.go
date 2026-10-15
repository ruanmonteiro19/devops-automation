package collectors

import (
	"context"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type integrationRoundTrip func(*http.Request) (*http.Response, error)

func (f integrationRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func mockProvider(fn func(*http.Request) string) Runner {
	return Runner{integrationClient: func() *http.Client {
		return &http.Client{Transport: integrationRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(fn(r))), Header: http.Header{}}, nil
		})}
	}}
}
func TestCloudAPIPaginationAndAuthentication(t *testing.T) {
	t.Setenv("OBSERVA_TEST_ACCESS_KEY_ID", "test-key")
	t.Setenv("OBSERVA_TEST_SECRET_ACCESS_KEY", "not-a-real-secret")
	t.Setenv("OBSERVA_TEST_SESSION_TOKEN", "temporary-token")
	calls := 0
	runner := mockProvider(func(r *http.Request) string {
		calls++
		if r.URL.Host != "ec2.us-east-1.amazonaws.com" || !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=test-key/") || r.Header.Get("X-Amz-Security-Token") != "temporary-token" {
			t.Fatal("missing signed request")
		}
		if calls == 1 {
			return `<DescribeInstancesResponse><nextToken>next</nextToken><reservationSet><item><instancesSet><item><instanceId>i-1</instanceId><instanceState><name>running</name></instanceState><placement><availabilityZone>us-east-1a</availabilityZone></placement><tagSet><item><key>Name</key><value>API</value></item></tagSet></item></instancesSet></item></reservationSet></DescribeInstancesResponse>`
		}
		if r.URL.Query().Get("NextToken") != "next" {
			t.Fatal("pagination missing")
		}
		return `<DescribeInstancesResponse/>`
	})
	result, e := runner.CollectIntegration(context.Background(), model.Integration{Kind: "aws", Region: "us-east-1", CredentialPrefix: "OBSERVA_TEST"})
	if e != nil || len(result.Resources) != 1 || result.Resources[0].Name != "API" || calls != 2 {
		t.Fatalf("AWS: %+v %v", result, e)
	}
	t.Setenv("OBSERVA_AZ_TENANT_ID", "tenant")
	t.Setenv("OBSERVA_AZ_CLIENT_ID", "client")
	t.Setenv("OBSERVA_AZ_CLIENT_SECRET", "secret")
	calls = 0
	runner = mockProvider(func(r *http.Request) string {
		calls++
		if calls == 1 {
			if r.URL.Host != "login.microsoftonline.com" || r.Method != "POST" {
				t.Fatal("Azure OAuth endpoint")
			}
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "client_credentials" {
				t.Fatal("OAuth flow")
			}
			return `{"access_token":"access"}`
		}
		if r.Header.Get("Authorization") != "Bearer access" {
			t.Fatal("Azure auth missing")
		}
		return `{"value":[{"id":"/subscriptions/sub/resourceGroups/rg/providers/type/r","name":"VM","type":"Microsoft.Compute/virtualMachines","location":"eastus"}]}`
	})
	result, e = runner.CollectIntegration(context.Background(), model.Integration{Kind: "azure", Scope: "sub", CredentialPrefix: "OBSERVA_AZ"})
	if e != nil || len(result.Resources) != 1 || result.Resources[0].Location != "eastus" {
		t.Fatalf("Azure %+v %v", result, e)
	}
	t.Setenv("OBSERVA_GCP_ACCESS_TOKEN", "gcp-access")
	calls = 0
	runner = mockProvider(func(r *http.Request) string {
		calls++
		if r.Header.Get("Authorization") != "Bearer gcp-access" || r.URL.Host != "compute.googleapis.com" {
			t.Fatal("GCP auth endpoint")
		}
		if calls == 1 {
			return `{"nextPageToken":"page2","items":{"zones/us-central1-a":{"instances":[{"id":"18446744073709551610","name":"instance","zone":"zones/us-central1-a","status":"RUNNING","networkInterfaces":[{"networkIP":"10.0.0.1"}]}]}}}`
		}
		if r.URL.Query().Get("pageToken") != "page2" {
			t.Fatal("GCP pagination")
		}
		return `{"items":{}}`
	})
	result, e = runner.CollectIntegration(context.Background(), model.Integration{Kind: "gcp", Scope: "project", CredentialPrefix: "OBSERVA_GCP"})
	if e != nil || len(result.Resources) != 1 || result.Resources[0].Attributes["instance_id"] != "18446744073709551610" {
		t.Fatalf("GCP %+v %v", result, e)
	}
}
func TestCloudErrorsDoNotBecomeEmptySuccessOrLeakSecrets(t *testing.T) {
	t.Setenv("OBSERVA_TEST_ACCESS_KEY_ID", "key")
	t.Setenv("OBSERVA_TEST_SECRET_ACCESS_KEY", "sensitive-secret")
	r := mockProvider(func(req *http.Request) string {
		return `<ErrorResponse><Message>sensitive-secret</Message></ErrorResponse>`
	})
	result, e := r.CollectIntegration(context.Background(), model.Integration{Kind: "aws", Region: "us-east-1", CredentialPrefix: "OBSERVA_TEST"})
	if e == nil || result.Status != "error" || strings.Contains(result.Error, "sensitive-secret") {
		t.Fatal("invalid provider response accepted or leaked")
	}
	t.Setenv("OBSERVA_GCP_ACCESS_TOKEN", "token")
	r = mockProvider(func(req *http.Request) string { return `{"items":{"zone":{"warning":{"code":"UNREACHABLE"}}}}` })
	if _, e = r.CollectIntegration(context.Background(), model.Integration{Kind: "gcp", Scope: "p", CredentialPrefix: "OBSERVA_GCP"}); e == nil {
		t.Fatal("partial inventory accepted")
	}
}
func TestDeviceExporterFlowAndPrivateTargets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/snmp" || r.URL.Query().Get("target") != "192.168.1.10" || r.URL.Query().Get("auth") != "switch_v3" {
			t.Fatal("wrong exporter request")
		}
		_, _ = io.WriteString(w, "sysName{sysName=\"switch-core\"} 1\nifHCInOctets{ifIndex=\"1\",ifName=\"Gi1\",ifAlias=\"Uplink\"} 1000\nifOperStatus{ifIndex=\"1\",ifName=\"Gi1\"} 1\n")
	}))
	defer server.Close()
	p := model.Integration{Kind: "snmp_exporter", Endpoint: server.URL, Target: "192.168.1.10", Module: "if_mib", Auth: "switch_v3"}
	if _, e := (Runner{}).CollectIntegration(context.Background(), p); e == nil {
		t.Fatal("private target allowed implicitly")
	}
	result, e := (Runner{AllowPrivate: true}).CollectIntegration(context.Background(), p)
	if e != nil || len(result.Resources) != 1 || result.Resources[0].Name != "switch-core" || len(result.Resources[0].Interfaces) != 1 || result.Resources[0].Interfaces[0].Metrics["ifHCInOctets"] != 1000 {
		t.Fatalf("device %+v %v", result, e)
	}
	if _, _, e := parseDeviceMetrics("snmp_scrape_duration_seconds 1\n", "x"); e == nil {
		t.Fatal("exporter internals treated as device data")
	}
}
func TestDeviceRatesRequireTwoSuccessesAndRejectCounterReset(t *testing.T) {
	now := time.Now()
	prior := model.IntegrationResult{SuccessAt: now.Add(-10 * time.Second), Resources: []model.IntegrationResource{{ID: "switch", Interfaces: []model.DeviceInterface{{Index: "1", Metrics: map[string]float64{"ifHCInOctets": 100}}}}}}
	current := model.IntegrationResult{SuccessAt: now, Resources: []model.IntegrationResource{{ID: "switch", Interfaces: []model.DeviceInterface{{Index: "1", Metrics: map[string]float64{"ifHCInOctets": 1100, "ifHighSpeed": 1000}}}}}}
	DeviceRates(&current, prior)
	m := current.Resources[0].Interfaces[0].Metrics
	if m["observa_in_bits_per_second"] != 800 || math.Abs(m["observa_in_utilization_percent"]-0.00008) > 1e-12 {
		t.Fatalf("rates %+v", m)
	}
	delete(m, "observa_in_bits_per_second")
	delete(m, "observa_in_utilization_percent")
	m["ifHCInOctets"] = 1
	DeviceRates(&current, prior)
	if _, ok := m["observa_in_bits_per_second"]; ok {
		t.Fatal("counter reset fabricated rate")
	}
}
