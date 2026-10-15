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
import {
  IntegrationManager,
  type IntegrationState,
} from "./integration-manager";
import { request } from "@/lib/api";
vi.mock("@/lib/api", () => ({ request: vi.fn() }));
let state: IntegrationState;
function mount(devices = false, demo = false, initialCreate = false) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <IntegrationManager
        demo={demo}
        devices={devices}
        initialCreate={initialCreate}
      />
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  state = { integrations: [], results: {} };
  vi.mocked(request).mockImplementation(async (_path, init) =>
    init?.method ? {} : state,
  );
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
describe("Integration configuration", () => {
  it("saves a cloud profile containing environment names rather than credentials", async () => {
    mount();
    fireEvent.click(screen.getByRole("button", { name: "CONECTAR CLOUD" }));
    fireEvent.change(screen.getByLabelText("NOME"), {
      target: { value: "PROD" },
    });
    fireEvent.click(screen.getByRole("button", { name: "SALVAR PERFIL" }));
    await waitFor(() =>
      expect(
        vi
          .mocked(request)
          .mock.calls.some(
            ([path, init]) =>
              path === "/api/v1/integrations" && init?.method === "POST",
          ),
      ).toBe(true),
    );
    const call = vi
      .mocked(request)
      .mock.calls.find(([_p, i]) => i?.method === "POST")!;
    const payload = JSON.parse(String(call[1]?.body));
    expect(payload.kind).toBe("aws");
    expect(payload.credential_prefix).toBe("OBSERVA_CLOUD");
    expect(payload.secret).toBeUndefined();
  });
  it("keeps previous inventory visible with a failure message", async () => {
    state = {
      integrations: [{ id: "aws", name: "PROD", kind: "aws", enabled: true }],
      results: {
        aws: {
          at: new Date().toISOString(),
          success_at: new Date().toISOString(),
          status: "error",
          error: "provider HTTP 403",
          truncated: false,
          resources: [
            {
              id: "i-1",
              name: "API",
              kind: "aws.ec2.instance",
              status: "running",
            },
          ],
        },
      },
    };
    mount();
    expect(await screen.findByText(/INVENTÁRIO ANTERIOR MANTIDO/)).toBeTruthy();
    expect(screen.getByText("API")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "COLETAR AGORA" }));
    await waitFor(() =>
      expect(request).toHaveBeenCalledWith("/api/v1/integrations/aws/collect", {
        method: "POST",
      }),
    );
  });
  it("opens a reviewable editor from quick creation", () => {
    mount(false, false, true);
    expect(screen.getByRole("dialog")).toBeTruthy();
    expect(screen.getByRole("button", { name: "SALVAR PERFIL" })).toBeTruthy();
    expect(
      vi
        .mocked(request)
        .mock.calls.some(([_path, init]) => init?.method === "POST"),
    ).toBe(false);
  });
  it("blocks demo network writes and external reads", () => {
    mount(true, true);
    expect(
      (
        screen.getByRole("button", {
          name: "ADICIONAR EQUIPAMENTO",
        }) as HTMLButtonElement
      ).disabled,
    ).toBe(true);
    expect(request).not.toHaveBeenCalled();
  });
});
