export type RUMApplication = {
  id: string;
  name: string;
  origins: string[];
  enabled: boolean;
  require_consent: boolean;
  consent_banner: boolean;
  replay_enabled: boolean;
  sample_rate: number;
  replay_sample_rate: number;
  masking: "all" | "allowlist";
  public_text_selectors: string[];
  block_selectors: string[];
  exclude_paths: string[];
  capture_path: boolean;
  identify_users: boolean;
  apdex_t_ms: number;
};
export type RUMEvent = {
  id: string;
  app_id: string;
  session_id: string;
  page_id: string;
  at: string;
  kind: string;
  name: string;
  url: string;
  duration_ms: number;
  value: number;
  status: number;
  error: boolean;
  browser: string;
  os: string;
  device: string;
  user_id?: string;
};
export type RUMSession = {
  apdex: number | null;
  id: string;
  app_id: string;
  started: string;
  last: string;
  pages: number;
  errors: number;
  browser: string;
  os: string;
  device: string;
  user: string;
};
export type RUMAnalysis = {
  minutes: number;
  pages: number;
  requests: number;
  errors: number;
  sessions_total: number;
  identified_users: number;
  apdex: number | null;
  apdex_t_ms: number;
  apdex_counts: { satisfied: number; tolerated: number; frustrated: number };
  vitals_p75: Record<string, number | null>;
  sessions: RUMSession[];
  events: RUMEvent[];
  retention_hours: number;
  note: string;
};
export type ReplayChunk = {
  id: string;
  app_id: string;
  session_id: string;
  page_id: string;
  at: string;
  events: any[];
};
export function newRUMApplication(): RUMApplication {
  return {
    id: `web-${Array.from(crypto.getRandomValues(new Uint8Array(4)), (n) => n.toString(16).padStart(2, "0")).join("")}`,
    name: "",
    origins: [],
    enabled: true,
    require_consent: true,
    consent_banner: true,
    replay_enabled: false,
    sample_rate: 1,
    replay_sample_rate: 0.1,
    masking: "all",
    public_text_selectors: [],
    block_selectors: [],
    exclude_paths: ["/login", "/auth", "/checkout", "/payment"],
    capture_path: false,
    identify_users: false,
    apdex_t_ms: 1000,
  };
}
export function vitalHealth(
  name: string,
  value: number | null | undefined,
): "good" | "needs-improvement" | "poor" | "unknown" {
  if (value == null) return "unknown";
  const limits: Record<string, [number, number]> = {
    LCP: [2500, 4000],
    INP: [200, 500],
    CLS: [0.1, 0.25],
  };
  const l = limits[name];
  return !l
    ? "unknown"
    : value <= l[0]
      ? "good"
      : value <= l[1]
        ? "needs-improvement"
        : "poor";
}
export function replayPages(chunks: ReplayChunk[]): Map<string, any[]> {
  const pages = new Map<string, any[]>();
  for (const c of chunks) {
    pages.set(c.page_id, [...(pages.get(c.page_id) || []), ...c.events]);
  }
  for (const [id, events] of pages) {
    events.sort((a, b) => a.timestamp - b.timestamp);
    const firstFull = events.findIndex((e) => e.type === 2);
    if (firstFull < 0) {
      pages.delete(id);
      continue;
    }
    const meta = events
      .slice(0, firstFull)
      .reverse()
      .find((e) => e.type === 4);
    const retained = events.slice(firstFull);
    pages.set(
      id,
      meta
        ? [meta, ...retained]
        : [
            {
              type: 4,
              timestamp: retained[0]!.timestamp,
              data: { href: "about:blank", width: 1280, height: 720 },
            },
            ...retained,
          ],
    );
  }
  return pages;
}
