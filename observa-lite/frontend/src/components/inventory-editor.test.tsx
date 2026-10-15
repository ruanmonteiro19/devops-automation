// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { InventoryEditor } from "./inventory-editor";
import { request, type Host } from "@/lib/api";
vi.mock("@/lib/api", () => ({ request: vi.fn() }));
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});
const host: Host = {
  id: "host-1",
  name: "Original",
  os: "Linux",
  addresses: [],
  last_seen: "",
  processes: [],
  connections: [],
};
describe("Inventory editing", () => {
  it("saves a custom name and a newly entered group", async () => {
    vi.mocked(request).mockResolvedValue({ saved: true });
    const saved = vi.fn();
    render(
      <InventoryEditor host={host} groups={["Existing"]} onSaved={saved} />,
    );
    fireEvent.change(screen.getByLabelText("Nome de exibição"), {
      target: { value: "Payments" },
    });
    fireEvent.change(screen.getByLabelText("Grupo de hosts"), {
      target: { value: "New group" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Salvar alterações" }));
    await waitFor(() => expect(saved).toHaveBeenCalled());
    expect(
      JSON.parse(vi.mocked(request).mock.calls[0]![1]!.body as string),
    ).toEqual({ name: "Payments", host_group: "New group" });
  });
  it("includes process identity to reject reused PIDs and surfaces failed saves", async () => {
    vi.mocked(request).mockRejectedValue(new Error("Processo mudou"));
    render(
      <InventoryEditor
        host={host}
        process={{
          pid: 42,
          name: "java",
          service: "orders",
          runtime: "java",
          rss_bytes: 0,
        }}
        groups={[]}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Salvar alterações" }));
    expect(await screen.findByText("Processo mudou")).toBeTruthy();
    expect(
      JSON.parse(vi.mocked(request).mock.calls[0]![1]!.body as string),
    ).toEqual({
      pid: 42,
      process_name: "java",
      original_name: "java",
      original_service: "orders",
    });
  });
});
