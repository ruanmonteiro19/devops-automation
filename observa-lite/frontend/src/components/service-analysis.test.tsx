// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  ServiceAnalysis,
  type ServiceAnalysisResult,
} from "./service-analysis";
import { request } from "@/lib/api";
vi.mock("@/lib/api", () => ({ request: vi.fn() }));
const group = {
  name: "TimeoutError",
  attributes: {
    "exception.type": "TimeoutError",
    "exception.message": "Upstream timeout",
  },
  count: 3,
  errors: 3,
  p50_ms: 40,
  p95_ms: 70,
  p99_ms: 70,
  total_duration_ms: 150,
  mean_duration_ms: 50,
  example_trace_ids: ["trace-timeout"],
};
const empty: ServiceAnalysisResult = {
  service_id: "svc:prod:api",
  from: "2026-10-15T10:00:00Z",
  to: "2026-10-15T11:00:00Z",
  count_unit: "observed spans",
  duration_meaning: "sum",
  observed_spans: 5,
  failures: [],
  endpoints: [],
  database_calls: [],
  outbound_calls: [],
  messaging: [],
  instances: [],
  versions: [],
  truncated: false,
  group_limit: 200,
};
const onTrace = vi.fn();
function mount(id = empty.service_id) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ServiceAnalysis
        serviceID={id}
        from={empty.from}
        to={empty.to}
        onTrace={onTrace}
      />
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  vi.mocked(request).mockResolvedValue({ ...empty, failures: [group] });
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
describe("ServiceAnalysis", () => {
  it("loads selected service and drills grouped failure to trace", async () => {
    mount();
    expect(await screen.findByText("Upstream timeout")).toBeTruthy();
    expect(request).toHaveBeenCalledWith(
      "/api/v1/apm/analysis?" +
        new URLSearchParams({
          service: empty.service_id,
          from: empty.from,
          to: empty.to,
        }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Abrir trace trace-timeout" }),
    );
    expect(onTrace).toHaveBeenCalledWith("trace-timeout");
    expect(screen.getByRole("tab", { name: "Falhas (1)" })).toBeTruthy();
  });
  it("renders database evidence and missing identifiers", async () => {
    vi.mocked(request).mockResolvedValue({
      ...empty,
      database_calls: [
        {
          ...group,
          name: "postgresql",
          attributes: {
            "db.system": "postgresql",
            "db.namespace": "",
            "db.operation": "SELECT",
          },
        },
      ],
    });
    mount();
    const tab = await screen.findByRole("tab", { name: "Banco de dados (1)" });
    fireEvent.mouseDown(tab, { button: 0, ctrlKey: false });
    fireEvent.click(tab);
    await waitFor(() =>
      expect(screen.getByText("postgresql", { selector: "p" })).toBeTruthy(),
    );
    expect(screen.getByText("Não informado")).toBeTruthy();
    expect(screen.getByText("SELECT")).toBeTruthy();
  });
  it("reports failures and retries", async () => {
    vi.mocked(request).mockRejectedValueOnce(new Error("analysis offline"));
    mount();
    expect(await screen.findByRole("alert")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Tentar novamente" }));
    expect(await screen.findByText("Upstream timeout")).toBeTruthy();
  });
  it("shows absent spans without claiming health", async () => {
    vi.mocked(request).mockResolvedValue(empty);
    mount();
    expect(
      await screen.findByText(
        "Nenhuma falha ou exceção observada neste período.",
      ),
    ).toBeTruthy();
    expect(screen.getByText(/CPU, memória e saúde/)).toBeTruthy();
  });
  it("disables calls without selected service", () => {
    mount("");
    expect(request).not.toHaveBeenCalled();
  });
  it("reports truncation", async () => {
    vi.mocked(request).mockResolvedValue({ ...empty, truncated: true });
    mount();
    expect(
      await screen.findByText(/Resultados limitados a 200 grupos/),
    ).toBeTruthy();
  });
});
