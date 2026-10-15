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
  APMProblemsPage,
  type APMProblem,
  type APMRule,
} from "./apm-problems-page";
import { api, request } from "@/lib/api";
vi.mock("@/lib/api", () => ({
  api: { apmServices: vi.fn() },
  request: vi.fn(),
}));
const rule: APMRule = {
  id: "rule-1",
  name: "API falhando",
  service_id: "svc:prod:api",
  metric: "error_rate",
  threshold: 5,
  min_requests: 10,
  window_minutes: 5,
  severity: "warning",
  enabled: true,
};
const problem: APMProblem = {
  id: "problem-1",
  rule_id: rule.id,
  service_id: rule.service_id,
  name: rule.name,
  severity: rule.severity,
  status: "active",
  opened_at: "2026-10-15T10:00:00Z",
  updated_at: "2026-10-15T10:01:00Z",
  value: 50,
  threshold: 5,
  metric: "error_rate",
  window_minutes: 5,
  evidence: ["20 requests, 10 errors"],
  example_trace_ids: ["trace-error"],
  timeline: [
    {
      at: "2026-10-15T10:00:00Z",
      state: "active",
      value: 50,
      summary: "threshold breached",
    },
    {
      at: "2026-10-15T10:01:00Z",
      state: "unknown",
      value: 0,
      summary: "No minimum sample; kept active",
    },
  ],
};
function mount(path = "correlacao", demo = false) {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <APMProblemsPage path={path} demo={demo} />
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  vi.mocked(api.apmServices).mockResolvedValue({
    services: [
      {
        id: rule.service_id,
        name: "api",
        namespace: "prod",
        instances: 1,
        span_count: 20,
        requests: 20,
        errors: 10,
        error_rate: 50,
        requests_per_second: 1,
        p50_ms: 10,
        p95_ms: 20,
        p99_ms: 20,
        endpoints: [],
      },
    ],
  });
  vi.mocked(request).mockImplementation(async (path) =>
    path.startsWith("/api/v1/apm/problems")
      ? { problems: [problem] }
      : { rules: [rule] },
  );
  vi.stubGlobal("crypto", { randomUUID: () => "new-rule" });
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});
describe("APMProblemsPage", () => {
  it("creates a rule with service identity and metric criteria", async () => {
    mount();
    await screen.findByText(rule.name);
    fireEvent.click(screen.getByRole("button", { name: "Nova regra" }));
    fireEvent.change(screen.getByLabelText("Nome"), {
      target: { value: "P95 elevado" },
    });
    fireEvent.change(screen.getByLabelText("Serviço"), {
      target: { value: rule.service_id },
    });
    fireEvent.change(screen.getByLabelText("Métrica"), {
      target: { value: "p95_ms" },
    });
    fireEvent.change(screen.getByLabelText("Limiar"), {
      target: { value: "250" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Salvar" }));
    await waitFor(() =>
      expect(request).toHaveBeenCalledWith("/api/v1/apm/rules", {
        method: "POST",
        body: JSON.stringify({
          ...rule,
          id: "new-rule",
          name: "P95 elevado",
          metric: "p95_ms",
          threshold: 250,
        }),
      }),
    );
  });
  it("shows missing-data problem without reporting recovery, evidence and timeline", async () => {
    mount("problemas-apm");
    expect(
      await screen.findByText(/Dados insuficientes na última avaliação/),
    ).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Detalhes" }));
    expect(screen.getByText("20 requests, 10 errors")).toBeTruthy();
    expect(screen.getByText("No minimum sample; kept active")).toBeTruthy();
    expect(
      screen
        .getByRole("link", { name: "Abrir trace trace-error" })
        .getAttribute("href"),
    ).toBe("/traces/trace-error");
    expect(screen.getByText(/ · Dados insuficientes$/)).toBeTruthy();
  });
  it("filters resolved problems", async () => {
    mount("problemas-apm");
    await screen.findByText(rule.name);
    fireEvent.change(screen.getByLabelText("Estado"), {
      target: { value: "resolved" },
    });
    await waitFor(() =>
      expect(request).toHaveBeenCalledWith(
        "/api/v1/apm/problems?status=resolved",
      ),
    );
  });
  it("requires confirmation before deleting a rule", async () => {
    mount();
    await screen.findByText(rule.name);
    fireEvent.click(screen.getByRole("button", { name: "Excluir" }));
    expect(request).not.toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({ method: "DELETE" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Confirmar exclusão" }));
    await waitFor(() =>
      expect(request).toHaveBeenCalledWith("/api/v1/apm/rules/rule-1", {
        method: "DELETE",
      }),
    );
  });
  it("prevents demo reads and writes", () => {
    mount("correlacao", true);
    expect(
      screen
        .getByRole("button", { name: "Nova regra" })
        .hasAttribute("disabled"),
    ).toBe(true);
    expect(
      screen
        .getByRole("button", { name: "Avaliar regras" })
        .hasAttribute("disabled"),
    ).toBe(true);
    expect(request).not.toHaveBeenCalled();
    expect(api.apmServices).not.toHaveBeenCalled();
  });
  it("shows query errors with retry", async () => {
    vi.mocked(request).mockRejectedValueOnce(new Error("offline"));
    mount();
    expect(await screen.findByRole("alert")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Tentar novamente" }));
    expect(await screen.findByText(rule.name)).toBeTruthy();
  });
});
