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
import { SLOPage } from "./slo-page";
import { api, request } from "@/lib/api";
vi.mock("@/lib/api", () => ({
  request: vi.fn(),
  api: { apmServices: vi.fn() },
}));
const mock = vi.mocked(request);
const services = vi.mocked(api.apmServices);
const unknown = {
  id: "checkout",
  name: "Checkout",
  service_id: "svc:prod:checkout",
  kind: "availability",
  target_percent: 100,
  total: 0,
  good: 0,
  bad: 0,
  success_percent: null,
  error_budget_remaining_percent: null,
  burn_rate: null,
  status: "unknown",
  basis: "request sample",
  from: "2026-10-15T10:00:00Z",
  to: "2026-10-15T11:00:00Z",
};
function mount(demo = false) {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <SLOPage demo={demo} />
    </QueryClientProvider>,
  );
}
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});
describe("SLO policies", () => {
  it("keeps unknown evaluations empty and burn undefined for 100% targets", async () => {
    mock.mockResolvedValue({ slos: [unknown] });
    mount();
    await screen.findByText("Checkout");
    expect(screen.getByText("Sem dados")).toBeTruthy();
    expect(screen.getAllByText("—")).toHaveLength(3);
    expect(screen.getByText(/Meta 100%: sem orçamento/)).toBeTruthy();
    expect(screen.queryByText("100%", { exact: true })).toBeNull();
  });
  it("creates a latency objective and refreshes results", async () => {
    mock.mockImplementation(async (_path, init) => (init ? {} : { slos: [] }));
    services.mockResolvedValue({
      services: [
        {
          id: "svc:prod:checkout",
          name: "checkout",
          namespace: "prod",
        } as never,
      ],
    });
    mount();
    await screen.findByText(/Nenhum SLO configurado/);
    fireEvent.click(screen.getByText("Novo SLO"));
    await screen.findByText("checkout (prod)");
    fireEvent.change(screen.getByLabelText("Nome"), {
      target: { value: "Fast checkout" },
    });
    fireEvent.change(screen.getByLabelText("Serviço"), {
      target: { value: "svc:prod:checkout" },
    });
    fireEvent.change(screen.getByLabelText("Tipo"), {
      target: { value: "latency" },
    });
    fireEvent.change(screen.getByLabelText("Limite de latência (ms)"), {
      target: { value: "250" },
    });
    fireEvent.click(screen.getByText("Salvar"));
    await waitFor(() =>
      expect(mock.mock.calls.some(([, init]) => init?.method === "POST")).toBe(
        true,
      ),
    );
    const post = mock.mock.calls.find(([, init]) => init?.method === "POST");
    expect(JSON.parse(String(post?.[1]?.body))).toMatchObject({
      name: "Fast checkout",
      kind: "latency",
      latency_threshold_ms: 250,
      service_id: "svc:prod:checkout",
    });
  });
  it("edits the existing identity and confirms deletion", async () => {
    mock.mockImplementation(async (_path, init) =>
      init ? {} : { slos: [unknown] },
    );
    services.mockResolvedValue({ services: [] });
    mount();
    await screen.findByText("Checkout");
    fireEvent.click(screen.getByText("Editar"));
    fireEvent.change(screen.getByLabelText("Nome"), {
      target: { value: "Changed" },
    });
    fireEvent.click(screen.getByText("Salvar"));
    await waitFor(() =>
      expect(mock.mock.calls.some(([, init]) => init?.method === "POST")).toBe(
        true,
      ),
    );
    expect(
      JSON.parse(
        String(
          mock.mock.calls.find(([, init]) => init?.method === "POST")?.[1]
            ?.body,
        ),
      ),
    ).toMatchObject({ id: "checkout", name: "Changed" });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    fireEvent.click(screen.getByText("Excluir"));
    expect(mock.mock.calls.some(([, init]) => init?.method === "DELETE")).toBe(
      false,
    );
    fireEvent.click(screen.getByText("Confirmar exclusão"));
    await waitFor(() =>
      expect(mock).toHaveBeenCalledWith("/api/v1/slos/checkout", {
        method: "DELETE",
      }),
    );
  });
  it("reports query errors and disables calls in demonstration", async () => {
    mount(true);
    expect(mock).not.toHaveBeenCalled();
    expect(services).not.toHaveBeenCalled();
    expect(screen.queryByText("Novo SLO")).toBeNull();
    cleanup();
    mock.mockRejectedValue(new Error("forbidden"));
    mount();
    await screen.findByRole("alert");
    expect(screen.getByText(/forbidden/)).toBeTruthy();
  });
});
