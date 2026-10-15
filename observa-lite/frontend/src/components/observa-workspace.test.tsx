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
import { ObservaApp } from "./observa-app";
import { api } from "@/lib/api";
import { createDemoSnapshot } from "@/lib/demo-data";
const location = vi.hoisted(() => ({ pathname: "/" }));
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, params, onClick, ...rest }: any) => (
    <a
      {...rest}
      href={`/${params._splat}`}
      onClick={(e: any) => {
        e.preventDefault();
        onClick?.(e);
      }}
    >
      {children}
    </a>
  ),
  useLocation: ({ select }: any) => select(location),
  useNavigate: () => vi.fn(),
}));
vi.mock("@/lib/api", async (importOriginal) => {
  const real = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...real,
    api: { ...real.api, snapshot: vi.fn(), createDataSource: vi.fn() },
  };
});
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}));
function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <ObservaApp />
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  localStorage.clear();
  vi.clearAllMocks();
  location.pathname = "/";
  vi.mocked(api.snapshot).mockResolvedValue(createDemoSnapshot());
});
afterEach(cleanup);
describe("Integrated workspace shell", () => {
  it("keeps collapsed navigation accessible and persists favorites", async () => {
    mount();
    await screen.findByText("Uma visão clara.");
    const favorite = screen.getByRole("button", {
      name: "Adicionar aos favoritos: Hosts",
    });
    fireEvent.click(favorite);
    expect(
      JSON.parse(localStorage.getItem("observa-favorites-v1") || "[]"),
    ).toContain("hosts");
    fireEvent.click(screen.getByRole("button", { name: "Recolher menu" }));
    expect(screen.getByRole("link", { name: "Hosts" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Central de ajuda" })).toBeTruthy();
  });
  it("edits an existing data source and does not require a secret", async () => {
    location.pathname = "/data-sources";
    const snapshot = createDemoSnapshot();
    snapshot.datasources = [
      {
        id: "source-1",
        name: "Prometheus",
        kind: "prometheus",
        url: "https://metrics.example",
      },
    ];
    vi.mocked(api.snapshot).mockResolvedValue(snapshot);
    vi.mocked(api.createDataSource).mockResolvedValue(snapshot.datasources[0]!);
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Editar fonte" }),
    );
    fireEvent.change(screen.getByLabelText("Nome"), {
      target: { value: "Prometheus production" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Salvar no servidor" }));
    await waitFor(() =>
      expect(api.createDataSource).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "source-1",
          name: "Prometheus production",
          url: "https://metrics.example",
        }),
        expect.anything(),
      ),
    );
  });
});
