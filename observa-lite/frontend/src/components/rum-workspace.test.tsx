// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RUMWorkspace } from "./rum-workspace";
import { request } from "@/lib/api";
vi.mock("@/lib/api", () => ({ request: vi.fn() }));
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, params, ...rest }: any) => (
    <a {...rest} href={`/${params._splat}`}>
      {children}
    </a>
  ),
}));
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});
function mount(demo = false) {
  const q = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={q}>
      <RUMWorkspace path="rum" demo={demo} />
    </QueryClientProvider>,
  );
}
describe("RUM workspace", () => {
  it("does not fabricate demo telemetry or write demo configuration", () => {
    mount(true);
    expect(screen.getByText(/não simula sessões/i)).toBeTruthy();
    expect(
      (
        screen.getByRole("button", {
          name: "Adicionar frontend",
        }) as HTMLButtonElement
      ).disabled,
    ).toBe(true);
    expect(request).not.toHaveBeenCalled();
  });
  it("creates a frontend with safe capture defaults and installation instructions", async () => {
    vi.mocked(request).mockImplementation(async (path, init) =>
      path === "/api/v1/rum/apps" && !init
        ? { applications: [] }
        : path.startsWith("/api/v1/rum/analysis")
          ? {
              sessions: [],
              events: [],
              vitals_p75: {},
              apdex_counts: { satisfied: 0, tolerated: 0, frustrated: 0 },
              apdex: null,
            }
          : {},
    );
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Configurar captura web" }),
    );
    fireEvent.change(screen.getByLabelText("Nome"), {
      target: { value: "Shop" },
    });
    fireEvent.change(screen.getByLabelText("Origens permitidas"), {
      target: { value: "https://shop.test" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Salvar frontend" }));
    await waitFor(() =>
      expect(
        vi.mocked(request).mock.calls.some((c) => c[1]?.method === "POST"),
      ).toBe(true),
    );
    const call = vi
      .mocked(request)
      .mock.calls.find((c) => c[1]?.method === "POST")!;
    const body = JSON.parse(call[1]!.body as string);
    expect(body.require_consent).toBe(true);
    expect(body.replay_enabled).toBe(false);
    expect(body.origins).toEqual(["https://shop.test"]);
    expect(
      await screen.findByRole("heading", { name: "Instalar captura web" }),
    ).toBeTruthy();
  });
});
