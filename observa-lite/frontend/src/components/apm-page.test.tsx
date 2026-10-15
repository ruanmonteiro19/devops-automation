// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  fireEvent,
  render,
  screen,
  cleanup,
  waitFor,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { APMPage } from "./apm-page";
import { api } from "@/lib/api";
vi.mock("@/components/service-analysis", () => ({
  ServiceAnalysis: () => null,
}));
vi.mock("@/components/trace-logs", () => ({ TraceLogs: () => null }));
vi.mock("recharts", () => ({
  LineChart: () => null,
  Line: () => null,
  XAxis: () => null,
  YAxis: () => null,
  Tooltip: () => null,
  ResponsiveContainer: () => null,
}));
vi.mock("@/lib/api", () => ({
  api: {
    apmServices: vi.fn(),
    apmTraces: vi.fn(),
    apmTrace: vi.fn(),
    apmTimeseries: vi.fn(),
  },
}));
const metrics = {
  requests: 1,
  errors: 1,
  error_rate: 100,
  requests_per_second: 0.1,
  p50_ms: 20,
  p95_ms: 20,
  p99_ms: 20,
};
const service = {
  id: "svc:prod:checkout",
  name: "checkout",
  namespace: "prod",
  instances: 1,
  span_count: 1,
  endpoints: [{ name: "POST /order", ...metrics }],
  ...metrics,
};
const span = {
  trace_id: "a".repeat(32),
  span_id: "b".repeat(16),
  name: "POST /order",
  service_name: "checkout",
  service_instance_id: "pod-1",
  kind: "SERVER",
  status: "error",
  start_time: "2026-10-15T10:00:00Z",
  end_time: "2026-10-15T10:00:00.020Z",
  attributes: { "http.response.status_code": 500 },
  scope: { name: "queue-sdk", version: "2.0" },
  links: [{ trace_id: "c".repeat(32), span_id: "d".repeat(16) }],
  events: [
    {
      name: "exception",
      time: "2026-10-15T10:00:00Z",
      attributes: { "exception.message": "payment rejected" },
    },
  ],
};
const trace = {
  trace_id: span.trace_id,
  start_time: span.start_time,
  end_time: span.end_time,
  duration_ms: 20,
  spans: [span],
  incomplete: false,
  completeness: "root_present",
  missing_parent_ids: [],
};
function mount(path = "servicos", demo = false, initialTrace = "") {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <APMPage
        path={path}
        demo={demo}
        globalSearch=""
        initialTrace={initialTrace}
      />
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  vi.mocked(api.apmServices).mockResolvedValue({ services: [service] });
  vi.mocked(api.apmTraces).mockResolvedValue({ traces: [trace] });
  vi.mocked(api.apmTrace).mockResolvedValue(trace);
  vi.mocked(api.apmTimeseries).mockResolvedValue({ timeseries: [] });
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
describe("APM investigation", () => {
  it("opens trace from a route-provided ID", async () => {
    mount("traces", false, trace.trace_id);
    await waitFor(() =>
      expect(api.apmTrace).toHaveBeenCalledWith(trace.trace_id),
    );
    expect(await screen.findByLabelText("Waterfall do trace")).toBeTruthy();
  });
  it("opens service, trace waterfall and exception evidence using the service identity", async () => {
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "checkout" }));
    await waitFor(() =>
      expect(api.apmTraces).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(String),
        service.id,
        false,
      ),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: new RegExp(span.trace_id) }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: /checkout POST \/order/ }),
    );
    expect(await screen.findByText("payment rejected")).toBeTruthy();
    expect(screen.getByText("500")).toBeTruthy();
    expect(screen.getByLabelText("Waterfall do trace")).toBeTruthy();
    expect(
      screen
        .getByRole("link", { name: `Trace: ${"c".repeat(32)}` })
        .getAttribute("href"),
    ).toBe(`/traces/${"c".repeat(32)}`);
    expect(screen.getByText(/queue-sdk/)).toBeTruthy();
  });
  it("shows API failures and supports retry", async () => {
    vi.mocked(api.apmServices).mockRejectedValueOnce(new Error("offline"));
    mount();
    expect(await screen.findByRole("alert")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Tentar novamente" }));
    expect(
      await screen.findByRole("button", { name: "checkout" }),
    ).toBeTruthy();
  });
  it("shows honest empty services", async () => {
    vi.mocked(api.apmServices).mockResolvedValue({ services: [] });
    mount();
    expect(
      await screen.findByText(/Nenhum serviço instrumentado/),
    ).toBeTruthy();
  });
  it("does not query live telemetry in demo", () => {
    mount("servicos", true);
    expect(screen.getByText(/APM indisponível/)).toBeTruthy();
    expect(api.apmServices).not.toHaveBeenCalled();
    expect(api.apmTraces).not.toHaveBeenCalled();
  });
  it("errors view requests failed traces", async () => {
    mount("erros");
    await waitFor(() =>
      expect(api.apmTraces).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(String),
        "",
        true,
      ),
    );
  });
});
