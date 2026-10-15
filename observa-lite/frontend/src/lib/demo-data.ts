import type { Snapshot } from "./api";

export function createDemoSnapshot(): Snapshot {
  const now = new Date().toISOString();
  return {
    generated_at: now,
    hosts: [
      { id: "demo-host-1", name: "app-linux-01", os: "Ubuntu 24.04", addresses: ["10.24.1.18"], last_seen: now, processes: [{ pid: 1842, name: "observa-api", service: "API de pedidos", runtime: "Go 1.24", rss_bytes: 142_606_336 }], connections: [{ pid: 1842, local_ip: "10.24.1.18", local_port: 8080, remote_ip: "10.24.2.11", remote_port: 5432, state: "ESTABLISHED" }] },
      { id: "demo-host-2", name: "db-linux-01", os: "Debian 12", addresses: ["10.24.2.11"], last_seen: now, processes: [{ pid: 922, name: "postgres", service: "PostgreSQL", runtime: "PostgreSQL 16", rss_bytes: 624_951_296 }], connections: [] },
    ],
    service_map: { nodes: [
      { id: "gateway", name: "Gateway HTTP", kind: "service", runtime: "Nginx", status: "ok", instances: 2 },
      { id: "api", name: "API de pedidos", kind: "service", runtime: "Go", status: "warning", instances: 3 },
      { id: "postgres", name: "PostgreSQL", kind: "database", host: "db-linux-01", status: "ok", instances: 1 },
    ], edges: [
      { id: "gw-api", from: "gateway", to: "api", kind: "http", count: 1284, evidence: "Conexões TCP e cabeçalhos HTTP observados", confidence: "illustrative" },
      { id: "api-db", from: "api", to: "postgres", kind: "tcp", count: 488, evidence: "Fluxos TCP na porta 5432", confidence: "illustrative" },
    ] },
    network_map: { nodes: [
      { id: "net-a", name: "app-linux-01", kind: "host", status: "ok" },
      { id: "net-b", name: "db-linux-01", kind: "host", status: "ok" },
    ], edges: [{ id: "net-ab", from: "net-a", to: "net-b", kind: "tcp/5432", count: 488, evidence: "Fluxos IP observados", confidence: "illustrative" }] },
    templates: [{ id: "demo-http", name: "Saúde da API", kind: "http", target: "https://api.exemplo.local/health", interval_seconds: 60, timeout_seconds: 5, expected_status: 200, enabled: true, last_run: now, last_value: 200, last_error: "", status: "ok" }],
    datasources: [{ id: "demo-prom", name: "Prometheus principal", kind: "prometheus", url: "http://prometheus:9090", secret_env: "PROMETHEUS_TOKEN" }],
    dashboards: [{ id: "demo-ops", title: "Operação Linux", panels: [{ id: "p1", title: "Hosts observados", kind: "stat", query: "2" }] }],
    events: [
      { id: "demo-event-1", at: now, severity: "alta", entity: "API de pedidos", summary: "Latência acima da linha de base", evidence: ["p95 acima de 800 ms", "32 requisições afetadas"], status: "aberto" },
      { id: "demo-event-2", at: now, severity: "média", entity: "app-linux-01", summary: "Memória residente em crescimento", evidence: ["RSS +18% em 30 min"], status: "investigando" },
    ],
  };
}