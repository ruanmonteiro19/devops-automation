package server

import (
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"net/http"
)

type extension struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Available   bool   `json:"available"`
	Configured  bool   `json:"configured"`
	Integration string `json:"integration,omitempty"`
	Version     string `json:"version"`
}

func extensionCatalog(sources []model.DataSource, templates []model.Template, agentCount int) []extension {
	configured := map[string]bool{}
	for _, s := range sources {
		configured[s.Kind] = true
	}
	for _, t := range templates {
		configured[t.Kind] = true
	}
	defs := []extension{
		{ID: "linux-agent", Name: "Linux Discovery Agent", Category: "APM", Description: "Descoberta passiva de processos, sockets e dependências", Available: true, Integration: "agent"},
		{ID: "prometheus", Name: "Prometheus", Category: "Métricas", Description: "Consultar métricas via PromQL", Available: true, Integration: "datasource"},
		{ID: "json", Name: "REST / JSON", Category: "Data Sources", Description: "Consultar endpoints JSON autorizados", Available: true, Integration: "datasource"},
		{ID: "http", Name: "HTTP Monitor", Category: "Monitoramento", Description: "Verificações ativas HTTP", Available: true, Integration: "template"},
		{ID: "tcp", Name: "TCP Monitor", Category: "Monitoramento", Description: "Verificações de conectividade TCP", Available: true, Integration: "template"},
		{ID: "mcp", Name: "MCP Server", Category: "IA e Integrações", Description: "Ferramentas MCP somente leitura", Available: true, Configured: true, Integration: "mcp"},
		{ID: "snmp", Name: "SNMP Exporter / IF-MIB", Category: "Rede", Description: "Leitura de equipamentos e interfaces via SNMP Exporter", Available: true, Integration: "devices"},
		{ID: "zabbix", Name: "Zabbix", Category: "Integrações", Description: "Importação de itens, eventos e triggers", Integration: "future"},
		{ID: "opentelemetry", Name: "OpenTelemetry", Category: "APM", Description: "Traces, logs e métricas OTLP HTTP com limites locais", Available: true, Integration: "otlp"},
		{ID: "kubernetes", Name: "Kubernetes / OpenShift", Category: "Cloud", Description: "Descoberta de workloads, namespaces e serviços", Integration: "future"},
		{ID: "oracle", Name: "Oracle Database", Category: "Banco de dados", Description: "Indicadores e sessões de bancos Oracle", Integration: "future"},
		{ID: "postgresql", Name: "PostgreSQL", Category: "Banco de dados", Description: "Métricas SQL e saúde do PostgreSQL", Integration: "future"},
		{ID: "sqlserver", Name: "SQL Server", Category: "Banco de dados", Description: "Métricas e desempenho do SQL Server", Integration: "future"},
		{ID: "elastic", Name: "Elasticsearch / OpenSearch", Category: "Logs", Description: "Consultas e correlação de logs", Integration: "future"},
		{ID: "loki", Name: "Grafana Loki", Category: "Logs", Description: "Consultar LogQL no Loki com janela e volume limitados", Available: true, Integration: "datasource"},
		{ID: "tempo", Name: "Grafana Tempo", Category: "Traces", Description: "Investigação de distributed traces", Integration: "future"},
		{ID: "jenkins", Name: "Jenkins", Category: "DevOps", Description: "Pipelines, builds e dados operacionais", Integration: "future"},
		{ID: "aws", Name: "AWS", Category: "Cloud", Description: "Inventário EC2 por região via API assinada", Available: true, Integration: "cloud"},
		{ID: "azure", Name: "Microsoft Azure", Category: "Cloud", Description: "Inventário Resource Manager por subscription", Available: true, Integration: "cloud"},
		{ID: "gcp", Name: "Google Cloud", Category: "Cloud", Description: "Inventário Compute Engine por projeto", Available: true, Integration: "cloud"},
		{ID: "oci", Name: "Oracle Cloud", Category: "Cloud", Description: "Telemetria de serviços OCI", Integration: "future"},
	}
	for i := range defs {
		defs[i].Version = "0.1"
		if defs[i].ID == "linux-agent" && agentCount > 0 {
			defs[i].Configured = true
		}
		if configured[defs[i].ID] {
			defs[i].Configured = true
		}
	}
	return defs
}
func (s *Server) extensions(w http.ResponseWriter, r *http.Request) {
	snapshot := s.Store.Snapshot()
	catalog := extensionCatalog(snapshot.DataSources, snapshot.Templates, len(snapshot.Hosts))
	for i := range catalog {
		if catalog[i].ID == "opentelemetry" {
			catalog[i].Configured = len(snapshot.ServiceMap.Nodes) > 0
		}
	}
	profiles := s.Store.Integrations()
	results := s.Store.IntegrationResults()
	for i := range catalog {
		for _, profile := range profiles {
			kind := profile.Kind
			if kind == "snmp_exporter" {
				kind = "snmp"
			}
			if catalog[i].ID == kind && !results[profile.ID].SuccessAt.IsZero() {
				catalog[i].Configured = true
			}
		}
	}
	write(w, 200, map[string]interface{}{"extensions": catalog})
}
func (s *Server) capabilities(w http.ResponseWriter, r *http.Request) {
	write(w, 200, map[string]interface{}{
		"version":        "0.2.0",
		"authentication": map[string]interface{}{"mode": "api_token", "sso": false, "mfa": false, "rbac": false, "description": "Autenticação administrativa por token. SSO e MFA dependem de implementação de backend e provedor de identidade."},
		"features": map[string]bool{
			"cloud_inventory_aws_ec2":          true,
			"cloud_inventory_azure_arm":        true,
			"cloud_inventory_gcp_compute":      true,
			"snmp_exporter_read":               true,
			"topology_views":                   true,
			"service_map":                      true, // Instrumented parent-child span relationships.
			"service_map_api":                  true,
			"process_mapping":                  true,
			"network_ip_flows":                 true,
			"network_physical_map":             false,
			"template_http_tcp":                true,
			"zabbix_lld":                       false,
			"datasource_prometheus_json":       true,
			"datasource_loki_read":             true,
			"dashboard_html_sandbox":           true,
			"grafana_dashboard_import":         false,
			"gateway_manual_routes":            true,
			"gateway_heartbeat":                false,
			"mcp_read_only":                    true,
			"oidc_sso":                         false,
			"mfa":                              false,
			"rbac":                             false,
			"snmp_network_discovery":           false,
			"traces":                           true,
			"otlp_http_json":                   true,
			"otlp_http_protobuf":               true,
			"otlp_grpc":                        false,
			"otlp_logs":                        true,
			"otlp_metrics_gauge_sum_histogram": true,
			"apm_service_analysis":             true,
			"slo_sample_evaluation":            true,
			"apm_threshold_rules":              true,
			"apm_red":                          true,
			"deep_apm":                         false,
			"ai_rca":                           false,
		},
	})
}
