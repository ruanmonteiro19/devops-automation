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
import { request } from "@/lib/api";
import { TraceLogs, traceLogWindow } from "./trace-logs";
vi.mock("@/lib/api", () => ({ request: vi.fn() }));
const start = new Date(Date.now() - 600000).toISOString(),
  end = new Date(Date.now() - 599000).toISOString();
function mount(disabled = false, traceStart = start, traceEnd = end) {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <TraceLogs
        traceID="trace-correlated"
        start={traceStart}
        end={traceEnd}
        disabled={disabled}
      />
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  vi.mocked(request).mockResolvedValue({
    logs: [
      {
        timestamp: start,
        service_name: "checkout",
        span_id: "span-error",
        severity_text: "ERROR",
        severity_number: 17,
        body: "payment failed",
      },
    ],
    truncated: false,
  });
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
describe("TraceLogs", () => {
  it("queries exact trace ID in a bounded window and displays evidence", async () => {
    mount();
    expect(await screen.findByText("payment failed")).toBeTruthy();
    expect(screen.getByText("Span: span-error")).toBeTruthy();
    expect(screen.getByText(/ERROR · checkout/)).toBeTruthy();
    const path = String(vi.mocked(request).mock.calls[0]?.[0]);
    const url = new URL(path, "http://localhost");
    expect(url.searchParams.get("trace_id")).toBe("trace-correlated");
    expect(url.searchParams.get("limit")).toBe("100");
    const from = Date.parse(url.searchParams.get("from")!),
      to = Date.parse(url.searchParams.get("to")!);
    expect(to - from).toBeLessThanOrEqual(86400000);
    expect(from).toBe(Date.parse(start) - 60000);
  });
  it("clamps a long trace window to last24h and now", () => {
    const now = Date.now();
    const range = traceLogWindow(
      new Date(now - 172800000).toISOString(),
      new Date(now + 3600000).toISOString(),
      now,
    )!;
    expect(Date.parse(range.from)).toBeGreaterThanOrEqual(now - 86400000);
    expect(Date.parse(range.to)).toBe(now);
    expect(Date.parse(range.to) - Date.parse(range.from)).toBeLessThanOrEqual(
      86400000,
    );
  });
  it("does not query expired traces", () => {
    mount(
      false,
      new Date(Date.now() - 172800000).toISOString(),
      new Date(Date.now() - 172799000).toISOString(),
    );
    expect(screen.getByText(/Trace fora da janela/)).toBeTruthy();
    expect(request).not.toHaveBeenCalled();
  });
  it("does not query in demo", () => {
    mount(true);
    expect(request).not.toHaveBeenCalled();
    expect(screen.getByText(/modo demonstração/)).toBeTruthy();
  });
  it("shows empty logs honestly", async () => {
    vi.mocked(request).mockResolvedValue({ logs: [], truncated: false });
    mount();
    expect(await screen.findByText(/Nenhum log correlacionado/)).toBeTruthy();
  });
  it("shows failure and retries", async () => {
    vi.mocked(request).mockRejectedValueOnce(new Error("logs offline"));
    mount();
    expect(await screen.findByRole("alert")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Tentar novamente" }));
    await waitFor(() =>
      expect(screen.getByText("payment failed")).toBeTruthy(),
    );
  });
});
