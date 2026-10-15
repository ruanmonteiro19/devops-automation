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
import { MonitorManager } from "./monitor-manager";
import { api, type MonitorTemplate } from "@/lib/api";
vi.mock("@/components/workspace-experience", () => ({
  WorkspaceHeading: ({ title }: any) => <h1>{title}</h1>,
}));
vi.mock("@/lib/api", () => ({
  api: { createTemplate: vi.fn(), deleteTemplate: vi.fn() },
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
const item: MonitorTemplate = {
  id: "health",
  name: "API health",
  kind: "http",
  target: "https://example.com/health",
  enabled: true,
  interval_seconds: 60,
  timeout_seconds: 5,
  expected_status: 200,
  last_run: "",
  last_value: 0,
  last_error: "",
  status: "unknown",
};
function mount(demo = false, initialCreate = false) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { mutations: { retry: false } } })
      }
    >
      <MonitorManager
        items={[item]}
        demo={demo}
        initialCreate={initialCreate}
      />
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.createTemplate).mockResolvedValue(item);
  vi.mocked(api.deleteTemplate).mockResolvedValue(undefined);
});
afterEach(cleanup);
describe("Monitor management", () => {
  it("edits the existing monitor instead of creating another identity", async () => {
    mount();
    fireEvent.click(screen.getByRole("button", { name: "Editar" }));
    fireEvent.change(screen.getByLabelText("Nome"), {
      target: { value: "API production" },
    });
    fireEvent.change(screen.getByLabelText("Intervalo (segundos)"), {
      target: { value: "120" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Salvar monitor" }));
    await waitFor(() =>
      expect(api.createTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "health",
          name: "API production",
          interval_seconds: 120,
        }),
        expect.anything(),
      ),
    );
  });
  it("pauses through the backend and requires confirmation before deletion", async () => {
    mount();
    fireEvent.click(screen.getByRole("button", { name: "Pausar" }));
    await waitFor(() =>
      expect(api.createTemplate).toHaveBeenCalledWith(
        expect.objectContaining({ id: "health", enabled: false }),
        expect.anything(),
      ),
    );
    await waitFor(() =>
      expect(
        (
          screen.getByRole("button", {
            name: "Excluir API health",
          }) as HTMLButtonElement
        ).disabled,
      ).toBe(false),
    );
    fireEvent.click(screen.getByRole("button", { name: "Excluir API health" }));
    expect(api.deleteTemplate).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Excluir monitor" }));
    await waitFor(() => expect(api.deleteTemplate).toHaveBeenCalled());
  });
  it("opens the editor from quick create and blocks demo mutations", () => {
    const view = mount(false, true);
    expect(screen.getByRole("dialog")).toBeTruthy();
    view.unmount();
    mount(true);
    expect(
      (
        screen.getByRole("button", {
          name: "Criar monitor",
        }) as HTMLButtonElement
      ).disabled,
    ).toBe(true);
  });
});
