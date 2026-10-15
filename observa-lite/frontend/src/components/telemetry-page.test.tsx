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
import { TelemetryPage } from "./telemetry-page";
import { request } from "@/lib/api";
vi.mock("@/lib/api", () => ({ request: vi.fn() }));
const mock = vi.mocked(request);
function mount(path = "telemetria-logs", demo = false) {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <TelemetryPage path={path} demo={demo} />
    </QueryClientProvider>,
  );
}
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});
describe("OTLP exploration", () => {
  it("queries an explicit bounded window and renders log context", async () => {
    mock.mockResolvedValue({
      logs: [
        {
          timestamp: "2026-10-15T10:00:00Z",
          service_name: "checkout",
          severity_text: "ERROR",
          severity_number: 17,
          body: { message: "Payment failed" },
          trace_id: "trace-123",
          span_id: "span-1",
          attributes: { tenant: "store-1" },
          resource_attributes: { host: "node-1" },
        },
      ],
      truncated: true,
    });
    mount();
    expect(mock).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("Serviço"), {
      target: { value: "checkout" },
    });
    fireEvent.change(screen.getByLabelText("Trace ID"), {
      target: { value: "trace-123" },
    });
    fireEvent.click(screen.getByText("Consultar"));
    await screen.findByText(/Payment failed/);
    expect(screen.getByText(/Trace: trace-123/)).toBeTruthy();
    expect(screen.getByText(/Resultado limitado/)).toBeTruthy();
    expect(screen.getByText(/store-1/)).toBeTruthy();
    const url = new URL(String(mock.mock.calls[0]?.[0]), "https://local");
    expect(url.searchParams.get("service")).toBe("checkout");
    expect(url.searchParams.get("trace_id")).toBe("trace-123");
    expect(
      Date.parse(url.searchParams.get("to")!) -
        Date.parse(url.searchParams.get("from")!),
    ).toBe(3600000);
  });
  it("preserves exact integers and histogram buckets without invented rates or percentiles", async () => {
    mock.mockResolvedValue({
      metrics: [
        {
          timestamp: "2026-10-15T10:00:00Z",
          service_name: "checkout",
          name: "requests",
          kind: "sum",
          unit: "{request}",
          temporality: "cumulative",
          monotonic: true,
          data: { asInt: "9007199254740993" },
        },
        {
          timestamp: "2026-10-15T10:00:00Z",
          service_name: "checkout",
          name: "latency",
          kind: "histogram",
          unit: "ms",
          temporality: "delta",
          monotonic: false,
          data: {
            count: "3",
            sum: 15,
            bucketCounts: ["1", "2", "0"],
            explicitBounds: [5, 10],
          },
        },
      ],
      truncated: false,
    });
    mount("metricas/itens-metricas");
    fireEvent.click(screen.getByText("Consultar"));
    await screen.findByText("9007199254740993");
    expect(screen.getByText("cumulative")).toBeTruthy();
    expect(screen.getByText("Buckets do histograma")).toBeTruthy();
    expect(screen.getByText(/bucketCounts/)).toBeTruthy();
    expect(screen.queryByText(/p95|p99|req\/s/i)).toBeNull();
  });
  it("disables real requests in demo", () => {
    mount("telemetria-logs", true);
    expect(screen.getByText(/consultas reais desativadas/)).toBeTruthy();
    expect(screen.queryByText("Consultar")).toBeNull();
    expect(mock).not.toHaveBeenCalled();
  });
  it("reports failures and retries then displays the empty state", async () => {
    mock
      .mockRejectedValueOnce(new Error("forbidden"))
      .mockResolvedValueOnce({ logs: [], truncated: false });
    mount();
    fireEvent.click(screen.getByText("Consultar"));
    await screen.findByRole("alert");
    expect(screen.getByText(/forbidden/)).toBeTruthy();
    fireEvent.click(screen.getByText("Tentar novamente"));
    await screen.findByText(/Nenhum log encontrado/);
    await waitFor(() => expect(mock).toHaveBeenCalledTimes(2));
  });
});
