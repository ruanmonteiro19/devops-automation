export type ProcessInfo = {
  display_name?: string;
  pid: number;
  name: string;
  service: string;
  runtime: string;
  rss_bytes: number;
};
export type ConnectionInfo = {
  pid: number;
  local_ip: string;
  local_port: number;
  remote_ip: string;
  remote_port: number;
  state: string;
};
export type Host = {
  id: string;
  name: string;
  os: string;
  addresses: string[];
  last_seen: string;
  host_group?: string;
  network_zone?: string;
  monitoring_mode?: string;
  processes: ProcessInfo[];
  connections: ConnectionInfo[];
};
export type MapNode = {
  id: string;
  name: string;
  kind: string;
  host?: string;
  runtime?: string;
  status: string;
  rss_bytes?: number;
  instances?: number;
};
export type MapEdge = {
  id: string;
  from: string;
  to: string;
  kind: string;
  count: number;
  evidence: string;
  confidence: string;
};
export type TopologyMap = { nodes: MapNode[]; edges: MapEdge[];interface_inventory?:Record<string,MapNode[]> };
export type MonitorTemplate = {
  id: string;
  name: string;
  kind: "http" | "tcp";
  target: string;
  interval_seconds: number;
  timeout_seconds: number;
  expected_status: number;
  enabled: boolean;
  last_run: string;
  last_value: number;
  last_error: string;
  status: string;
};
export type DataSource = {
  id: string;
  name: string;
  kind: "prometheus" | "json" | "loki";
  url: string;
  secret_env?: string;
};
export type DashboardPanel = {
  width?: "half" | "full";
  id: string;
  title: string;
  kind: string;
  source_id?: string;
  query?: string;
  html?: string;
  css?: string;
  js?: string;
};
export type Dashboard = { id: string; title: string; panels: DashboardPanel[] };
export type ObservaEvent = {
  id: string;
  at: string;
  severity: string;
  entity: string;
  summary: string;
  evidence: string[];
  status: string;
};
export type LogLine = {
  at: string;
  line: string;
  labels: Record<string, string>;
};
export type LogQueryResult = {
  logs: LogLine[];
  source: string;
  minutes: number;
  limit: number;
  storage: string;
  persisted_by_observa: boolean;
};
export type CatalogExtension = {
  id: string;
  name: string;
  category: string;
  description: string;
  available: boolean;
  configured: boolean;
  integration: string;
  version: string;
};
export type GatewayEndpoint = {
  id: string;
  network_zone: string;
  url: string;
  priority: number;
  enabled: boolean;
};
export type Snapshot = {
  generated_at: string;
  hosts: Host[];
  service_map: TopologyMap;
  process_map?: TopologyMap;
  network_map: TopologyMap;
  templates: MonitorTemplate[];
  datasources: DataSource[];
  dashboards: Dashboard[];
  events: ObservaEvent[];
};

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

const TOKEN_KEY = "observa-lite-api-token";
export const readToken = () =>
  typeof window === "undefined"
    ? ""
    : (window.sessionStorage.getItem(TOKEN_KEY) ?? "");
export const saveToken = (token: string) => {
  if (typeof window !== "undefined")
    token
      ? window.sessionStorage.setItem(TOKEN_KEY, token)
      : window.sessionStorage.removeItem(TOKEN_KEY);
};

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), 10_000);
  const token = readToken();
  try {
    const response = await fetch(path, {
      ...init,
      signal: controller.signal,
      headers: {
        "Content-Type": "application/json",
        ...(token ? { "X-API-Key": token } : {}),
        ...(window.sessionStorage.getItem("vetra-csrf")
          ? { "X-CSRF-Token": window.sessionStorage.getItem("vetra-csrf")! }
          : {}),
        ...init?.headers,
      },
    });
    if (!response.ok) {
      let detail = [400, 403, 429].includes(response.status)
        ? (await response.text()).slice(0, 300)
        : "";
      try {
        const parsed = JSON.parse(detail);
        if (typeof parsed.error === "string") detail = parsed.error;
      } catch {
        /* plain text API errors */
      }
      throw new ApiError(
        response.status,
        response.status === 401
          ? path === "/api/v1/auth/login"
            ? "Usuário ou senha inválidos"
            : "Token inválido ou ausente"
          : detail || `Falha na API (${response.status})`,
      );
    }
    if (response.status === 204) return undefined as T;
    const result = await response.json();
    if (typeof result?.csrf === "string")
      window.sessionStorage.setItem("vetra-csrf", result.csrf);
    return result as T;
  } catch (error) {
    if (error instanceof ApiError) throw error;
    if (error instanceof DOMException && error.name === "AbortError")
      throw new ApiError(408, "O servidor excedeu o tempo de resposta");
    throw new ApiError(0, "Servidor Vetra Observability não alcançável");
  } finally {
    window.clearTimeout(timeout);
  }
}

export type APMMetrics = {
  requests: number;
  errors: number;
  error_rate: number;
  requests_per_second: number;
  p50_ms: number;
  p95_ms: number;
  p99_ms: number;
};
export type APMService = APMMetrics & {
  id: string;
  name: string;
  namespace: string;
  instances: number;
  span_count: number;
  endpoints: (APMMetrics & { name: string })[];
};
export type APMSpan = {
  trace_state?: string;
  flags?: number;
  scope?: {
    name?: string;
    version?: string;
    attributes?: Record<string, unknown>;
  };
  scope_schema_url?: string;
  resource_schema_url?: string;
  links?: {
    trace_id: string;
    span_id: string;
    trace_state?: string;
    flags?: number;
    attributes?: Record<string, unknown>;
    dropped_attributes_count?: number;
  }[];
  trace_id: string;
  span_id: string;
  parent_span_id?: string;
  name: string;
  service_name: string;
  service_namespace?: string;
  service_instance_id?: string;
  kind: string;
  status: string;
  status_message?: string;
  start_time: string;
  end_time: string;
  attributes?: Record<string, unknown>;
  resource_attributes?: Record<string, unknown>;
  events?: {
    name: string;
    time: string;
    attributes?: Record<string, unknown>;
  }[];
};
export type APMTrace = {
  trace_id: string;
  start_time: string;
  end_time: string;
  duration_ms: number;
  spans: APMSpan[];
  incomplete: boolean;
  completeness: string;
  missing_parent_ids: string[];
};

export type APMTimeseries = {
  service_id: string;
  name: string;
  namespace: string;
  buckets: (APMMetrics & { start_time: string; end_time: string })[];
};

export const api = {
  apmTimeseries: (from: string, to: string) =>
    request<{ timeseries: APMTimeseries[] }>(
      "/api/v1/apm/timeseries?" + new URLSearchParams({ from, to }),
    ),
  apmServices: (from: string, to: string) =>
    request<{ services: APMService[] }>(
      "/api/v1/apm/services?" + new URLSearchParams({ from, to }),
    ),
  apmTraces: (from: string, to: string, service: string, failed: boolean) =>
    request<{ traces: APMTrace[] }>(
      "/api/v1/apm/traces?" +
        new URLSearchParams({
          from,
          to,
          ...(service ? { service } : {}),
          ...(failed ? { failed: "true" } : {}),
        }),
    ),
  apmTrace: (id: string) =>
    request<APMTrace>("/api/v1/apm/traces/" + encodeURIComponent(id)),
  snapshot: () => request<Snapshot>("/api/v1/snapshot"),
  gateways: () => request<{ gateways: GatewayEndpoint[] }>("/api/v1/gateways"),
  saveGateway: (value: GatewayEndpoint) =>
    request<{ saved: boolean }>("/api/v1/gateways", {
      method: "POST",
      body: JSON.stringify(value),
    }),
  removeGateway: (id: string) =>
    request<{ deleted: boolean }>(
      "/api/v1/gateways/" + encodeURIComponent(id),
      { method: "DELETE" },
    ),
  extensions: () =>
    request<{ extensions: CatalogExtension[] }>("/api/v1/extensions"),
  logs: (source: string, query: string, minutes = 60, limit = 100) =>
    request<LogQueryResult>(
      "/api/v1/logs?" +
        new URLSearchParams({
          source,
          query,
          minutes: String(minutes),
          limit: String(limit),
        }),
    ),
  createTemplate: (value: MonitorTemplate) =>
    request<MonitorTemplate>("/api/v1/templates", {
      method: "POST",
      body: JSON.stringify(value),
    }),
  deleteTemplate: (id: string) =>
    request<void>(`/api/v1/templates/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),
  createDataSource: (value: DataSource) =>
    request<DataSource>("/api/v1/datasources", {
      method: "POST",
      body: JSON.stringify(value),
    }),
  createDashboard: (value: Dashboard) =>
    request<Dashboard>("/api/v1/dashboards", {
      method: "POST",
      body: JSON.stringify(value),
    }),
  query: (source: string, expr: string) =>
    request<unknown>(
      `/api/v1/query?source=${encodeURIComponent(source)}&expr=${encodeURIComponent(expr)}`,
    ),
};
