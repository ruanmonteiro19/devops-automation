// @vitest-environment jsdom
import { afterEach, beforeEach, describe, it, expect, vi } from "vitest";
import {
  render,
  screen,
  fireEvent,
  cleanup,
  waitFor,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { LatestDataPage, type LatestItem } from "./latest-data";
import { request } from "@/lib/api";
vi.mock("@/lib/api", () => ({ request: vi.fn() }));
const item: LatestItem = {
  id: "11111111111111111111111111111111",
  host: "Core switch",
  host_id: "core",
  group: "Datacenter",
  name: "Interface status",
  key: "network.snmp.ifOperStatus",
  value: "1",
  unit: "",
  source: "SNMP",
  status: "Disponível",
  timestamp: new Date().toISOString(),
  change: "0",
  numeric: true,
  history: true,
};
function mount(demo = false) {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <LatestDataPage demo={demo} />
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  vi.mocked(request).mockImplementation(async (path) =>
    String(path).includes("/history")
      ? { items: [] }
      : { items: [item], total: 1 },
  );
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
describe("Latest data", () => {
  it("applies filters explicitly and clears them without rewriting configuration", async () => {
    mount();
    await screen.findByText("Core switch");
    fireEvent.change(screen.getByLabelText("Host ou equipamento"), {
      target: { value: "Core" },
    });
    expect(request).toHaveBeenCalledTimes(1);
    fireEvent.click(
      screen.getByRole("button", { name: "Aplicar", exact: true }),
    );
    await waitFor(() =>
      expect(
        vi
          .mocked(request)
          .mock.calls.some(([path]) => String(path).includes("host=Core")),
      ).toBe(true),
    );
    fireEvent.click(screen.getByRole("button", { name: "Limpar filtros" }));
    expect(
      (screen.getByLabelText("Host ou equipamento") as HTMLInputElement).value,
    ).toBe("");
    expect(
      vi.mocked(request).mock.calls.every(([, init]) => !init?.method),
    ).toBe(true);
  });
  it("opens the exact item history and shows the empty period honestly", async () => {
    mount();
    await screen.findByText("Core switch");
    fireEvent.click(
      screen.getByRole("button", { name: "Histórico", exact: true }),
    );
    await screen.findByText("Não há amostras no período selecionado.");
    expect(
      vi
        .mocked(request)
        .mock.calls.some(([path]) =>
          String(path).includes(`/latest-data/${item.id}/history?`),
        ),
    ).toBe(true);
  });
  it("does not issue production reads in demo mode", () => {
    mount(true);
    expect(request).not.toHaveBeenCalled();
    expect(screen.getByText(/Modo demonstração/)).toBeTruthy();
  });
});
