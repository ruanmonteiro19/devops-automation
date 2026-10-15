// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WorkspaceStudio } from "./workspace-studio";
import { request } from "../lib/api";
vi.mock("../lib/api", () => ({ request: vi.fn() }));
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});
function mount(path: string, demo = false) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <WorkspaceStudio path={path} demo={demo} />
    </QueryClientProvider>,
  );
}
it("creates notebook cells and renders actual query results", async () => {
  vi.mocked(request).mockImplementation(async (path) =>
    path === "/api/v1/vql"
      ? { rows: [{ name: "Production host" }] }
      : { objects: [], runs: [] },
  );
  mount("notebooks");
  fireEvent.click(screen.getByRole("button", { name: "Novo" }));
  fireEvent.change(screen.getByLabelText("Nome"), {
    target: { value: "Investigação" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Executar célula" }));
  expect(await screen.findByText("Production host")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Salvar" }));
  await waitFor(() =>
    expect(request).toHaveBeenCalledWith(
      "/api/v1/workspace",
      expect.objectContaining({
        method: "POST",
        body: expect.stringContaining('"kind":"notebook"'),
      }),
    ),
  );
});
it("keeps preview disconnected and performs no writes in demo", () => {
  mount("workflows", true);
  expect(
    (screen.getByRole("button", { name: "Novo" }) as HTMLButtonElement)
      .disabled,
  ).toBe(true);
  expect(request).not.toHaveBeenCalled();
});
it("converts frontend evidence into a consent-protected RUM application", async () => {
  vi.mocked(request).mockImplementation(async (path) =>
    path === "/api/v1/rum/apps"
      ? { applications: [] }
      : path === "/api/v1/frontends/discovered"
        ? { frontends: [{ id: "web-test", service: "Portal", port: 8080 }] }
        : { objects: [], runs: [] },
  );
  mount("frontends");
  fireEvent.click(
    await screen.findByRole("button", { name: "Configurar RUM · Portal:8080" }),
  );
  fireEvent.change(screen.getByLabelText("Origem pública da aplicação"), {
    target: { value: "https://portal.example" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Criar aplicação RUM" }));
  await waitFor(() =>
    expect(request).toHaveBeenCalledWith(
      "/api/v1/rum/apps",
      expect.objectContaining({
        body: expect.stringContaining('"require_consent":true'),
      }),
    ),
  );
  const call = vi
    .mocked(request)
    .mock.calls.find((c) => c[0] === "/api/v1/rum/apps" && c[1]?.method === "POST");
  const app = JSON.parse(String(call?.[1]?.body));
  expect(app.origins).toEqual(["https://portal.example"]);
  expect(app.masking).toBe("all");
  expect(app.replay_enabled).toBe(false);
});
