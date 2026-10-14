export type ProcessInfo = { pid: number; name: string; service: string; runtime: string; rss_bytes: number };
export type ConnectionInfo = { pid: number; local_ip: string; local_port: number; remote_ip: string; remote_port: number; state: string };
export type Host = { id: string; name: string; os: string; addresses: string[]; last_seen: string; processes: ProcessInfo[]; connections: ConnectionInfo[] };
export type MapNode = { id: string; name: string; kind: string; host?: string; runtime?: string; status: string; rss_bytes?: number; instances?: number };
export type MapEdge = { id: string; from: string; to: string; kind: string; count: number; evidence: string; confidence: string };
export type TopologyMap = { nodes: MapNode[]; edges: MapEdge[] };
export type MonitorTemplate = { id: string; name: string; kind: "http" | "tcp"; target: string; interval_seconds: number; timeout_seconds: number; expected_status: number; enabled: boolean; last_run: string; last_value: number; last_error: string; status: string };
export type DataSource = { id: string; name: string; kind: "prometheus" | "json"; url: string; secret_env?: string };
export type DashboardPanel = { id: string; title: string; kind: string; source_id?: string; query?: string; html?: string; css?: string; js?: string };
export type Dashboard = { id: string; title: string; panels: DashboardPanel[] };
export type ObservaEvent = { id: string; at: string; severity: string; entity: string; summary: string; evidence: string[]; status: string };
export type Snapshot = { generated_at: string; hosts: Host[]; service_map: TopologyMap; network_map: TopologyMap; templates: MonitorTemplate[]; datasources: DataSource[]; dashboards: Dashboard[]; events: ObservaEvent[] };

export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message); }
}

const TOKEN_KEY = "observa-lite-api-token";
export const readToken = () => typeof window === "undefined" ? "" : window.sessionStorage.getItem(TOKEN_KEY) ?? "";
export const saveToken = (token: string) => { if (typeof window !== "undefined") token ? window.sessionStorage.setItem(TOKEN_KEY, token) : window.sessionStorage.removeItem(TOKEN_KEY); };

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), 10_000);
  const token = readToken();
  try {
    const response = await fetch(path, {
      ...init,
      signal: controller.signal,
      headers: { "Content-Type": "application/json", ...(token ? { "X-API-Key": token } : {}), ...init?.headers },
    });
    if (!response.ok) throw new ApiError(response.status, response.status === 401 ? "Token inválido ou ausente" : `Falha na API (${response.status})`);
    if (response.status === 204) return undefined as T;
    return await response.json() as T;
  } catch (error) {
    if (error instanceof ApiError) throw error;
    if (error instanceof DOMException && error.name === "AbortError") throw new ApiError(408, "O servidor excedeu o tempo de resposta");
    throw new ApiError(0, "Servidor Observa Lite não alcançável");
  } finally { window.clearTimeout(timeout); }
}

export const api = {
  snapshot: () => request<Snapshot>("/api/v1/snapshot"),
  createTemplate: (value: MonitorTemplate) => request<MonitorTemplate>("/api/v1/templates", { method: "POST", body: JSON.stringify(value) }),
  deleteTemplate: (id: string) => request<void>(`/api/v1/templates/${encodeURIComponent(id)}`, { method: "DELETE" }),
  createDataSource: (value: DataSource) => request<DataSource>("/api/v1/datasources", { method: "POST", body: JSON.stringify(value) }),
  createDashboard: (value: Dashboard) => request<Dashboard>("/api/v1/dashboards", { method: "POST", body: JSON.stringify(value) }),
  query: (source: string, expr: string) => request<unknown>(`/api/v1/query?source=${encodeURIComponent(source)}&expr=${encodeURIComponent(expr)}`),
};