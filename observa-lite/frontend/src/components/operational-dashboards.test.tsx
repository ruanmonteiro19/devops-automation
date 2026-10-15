// @vitest-environment jsdom
import { afterEach, describe, it, expect, vi } from "vitest";
import {
  cleanup,
  render,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { OperationalDashboards } from "./operational-dashboards";
import { api, request, type Dashboard } from "@/lib/api";
vi.mock("@/lib/api", () => ({
  request: vi.fn(),
  api: {
    apmServices: vi.fn(),
    apmTimeseries: vi.fn(),
    createDashboard: vi.fn(),
  },
}));
vi.mock("recharts", () => ({
  ResponsiveContainer: () => null,
  LineChart: () => null,
  Line: () => null,
  XAxis: () => null,
  YAxis: () => null,
  Tooltip: () => null,
}));
const dashboard: Dashboard = {
  id: "ops",
  title: "Operations",
  panels: [
    {
      id: "requests",
      title: "Checkout requests",
      kind: "apm-stat",
      source_id: "observa-apm",
      query: JSON.stringify({
        service_id: "svc:prod:checkout",
        metric: "requests",
      }),
    },
  ],
};
const service = {
  id: "svc:prod:checkout",
  name: "checkout",
  namespace: "prod",
  requests: 42,
  error_rate: 12,
  p50_ms: 5,
  p95_ms: 10,
  p99_ms: 15,
  requests_per_second: 0.2,
};
function mount(items = [dashboard], demo = false) {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <OperationalDashboards items={items} demo={demo} />
    </QueryClientProvider>,
  );
}
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});
describe("Operational dashboards", () => {
  it("renders live values and refreshes the actual time range", async () => {
    vi.mocked(api.apmServices)
      .mockResolvedValueOnce({ services: [service as never] })
      .mockResolvedValueOnce({
        services: [{ ...service, requests: 74 } as never],
      });
    mount();
    await screen.findByText("42");
    fireEvent.change(screen.getByLabelText("Intervalo"), {
      target: { value: "15" },
    });
    fireEvent.click(screen.getByText("Atualizar"));
    await screen.findByText("74");
    const call = vi.mocked(api.apmServices).mock.calls[1]!;
    expect(Date.parse(call[1]) - Date.parse(call[0])).toBe(900000);
  });
  it("creates panels and preserves unsupported legacy configuration while editing", async () => {
    vi.mocked(api.apmServices).mockResolvedValue({
      services: [service as never],
    });
    vi.mocked(api.createDashboard).mockResolvedValue(dashboard);
    const legacy = {
      id: "legacy",
      title: "Legacy",
      kind: "custom-unknown",
      query: "original",
      js: "preserve",
    };
    mount([{ ...dashboard, panels: [legacy] }]);
    fireEvent.click(screen.getByText("Editar"));
    fireEvent.change(screen.getByLabelText("Título do dashboard"), {
      target: { value: "Revised" },
    });
    fireEvent.click(screen.getByText("Adicionar painel"));
    await screen.findByText("checkout (prod)");
    fireEvent.change(screen.getByLabelText("Serviço 2"), {
      target: { value: service.id },
    });
    fireEvent.change(screen.getByLabelText("Visualização 2"), {
      target: { value: "apm-series" },
    });
    fireEvent.change(screen.getByLabelText("Métrica 2"), {
      target: { value: "p95_ms" },
    });
    fireEvent.click(screen.getByText("Salvar"));
    await waitFor(() => expect(api.createDashboard).toHaveBeenCalled());
    const saved = vi.mocked(api.createDashboard).mock.calls[0]![0];
    expect(saved.title).toBe("Revised");
    expect(saved.panels[0]).toEqual(legacy);
    expect(saved.panels[1]).toMatchObject({
      kind: "apm-series",
      source_id: "observa-apm",
    });
    expect(JSON.parse(saved.panels[1]!.query!)).toEqual({
      service_id: service.id,
      metric: "p95_ms",
    });
  });
  it("requires confirmation before deleting", async () => {
    vi.mocked(api.apmServices).mockResolvedValue({ services: [] });
    vi.mocked(request).mockResolvedValue({});
    mount();
    fireEvent.click(screen.getByText("Excluir"));
    expect(request).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText("Confirmar exclusão"));
    await waitFor(() =>
      expect(request).toHaveBeenCalledWith("/api/v1/dashboards/ops", {
        method: "DELETE",
      }),
    );
  });
  it("does not fabricate empty values or query in demo", async () => {
    vi.mocked(api.apmServices).mockResolvedValue({ services: [] });
    mount();
    await screen.findByText("—");
    cleanup();
    vi.clearAllMocks();
    mount([dashboard], true);
    expect(screen.getByText("—")).toBeTruthy();
    expect(api.apmServices).not.toHaveBeenCalled();
    expect(api.apmTimeseries).not.toHaveBeenCalled();
    expect(screen.queryByText("Editar")).toBeNull();
  });
  it("isolates legacy HTML without scripts and labels literal stats", () => {
    mount([
      {
        id: "old",
        title: "Old",
        panels: [
          {
            id: "html",
            title: "Isolated",
            kind: "html",
            html: "<script>unsafe()</script>",
          },
          { id: "stat", title: "Literal", kind: "stat", query: "17" },
        ],
      },
    ]);
    expect(screen.getByTitle("Isolated").getAttribute("sandbox")).toBe("");
    expect(screen.getByText("Valor estático configurado")).toBeTruthy();
    expect(screen.getByText("17")).toBeTruthy();
  });
});
