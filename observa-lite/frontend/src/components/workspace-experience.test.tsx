// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import {
  WorkspaceTools,
  WorkspacePreferences,
  HelpCenter,
} from "./workspace-experience";
import { createDemoSnapshot } from "@/lib/demo-data";
import { loadAppearance } from "@/lib/workspace";
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
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute("data-theme");
});
afterEach(cleanup);
describe("Workspace experience", () => {
  it("opens creation actions with real editor destinations", () => {
    render(
      <WorkspaceTools
        snapshot={createDemoSnapshot()}
        demo={false}
        current="servicos"
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Criar" }));
    expect(
      screen
        .getByRole("link", { name: /Criar dashboard/ })
        .getAttribute("href"),
    ).toBe("/dashboards/novo");
    expect(
      screen
        .getByRole("link", { name: /Definir objetivo SLO/ })
        .getAttribute("href"),
    ).toBe("/slos/novo");
    expect(
      screen
        .getByRole("link", { name: /Criar regra de alerta/ })
        .getAttribute("href"),
    ).toBe("/correlacao/novo");
  });
  it("supports keyboard navigation search and closes after selection", () => {
    render(<WorkspaceTools demo={false} current="" />);
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    fireEvent.change(
      screen.getByRole("textbox", { name: "Buscar ações e páginas" }),
      { target: { value: "Central de ajuda" } },
    );
    fireEvent.click(screen.getByRole("link", { name: /Central de ajuda/ }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });
  it("applies and persists theme, accent and density", () => {
    render(<WorkspacePreferences />);
    fireEvent.click(screen.getByRole("button", { name: "Claro" }));
    fireEvent.click(screen.getByRole("button", { name: "Ciano" }));
    fireEvent.click(screen.getByRole("button", { name: "Compacta" }));
    expect(loadAppearance()).toEqual({
      theme: "light",
      accent: "cyan",
      density: "compact",
    });
    expect(document.documentElement.dataset["theme"]).toBe("light");
    expect(document.documentElement.dataset["density"]).toBe("compact");
  });
  it("tolerates corrupted preferences", () => {
    localStorage.setItem("observa-appearance-v1", "invalid");
    expect(loadAppearance().theme).toBe("dark");
  });
  it("filters help and provides an explicit collector fragment without credentials", () => {
    const snapshot = createDemoSnapshot();
    const view = render(<HelpCenter snapshot={snapshot} demo={false} />);
    fireEvent.change(screen.getByRole("textbox", { name: "Buscar ajuda" }), {
      target: { value: "OpenTelemetry" },
    });
    expect(
      screen.getByRole("link", { name: /Conectar OpenTelemetry/ }),
    ).toBeTruthy();
    expect(
      screen.queryByRole("link", { name: /Criar e editar dashboards/ }),
    ).toBeNull();
    view.rerender(
      <HelpCenter path="ajuda/otlp" snapshot={snapshot} demo={false} />,
    );
    expect(screen.getByText(/Trecho para mesclar/)).toBeTruthy();
    expect(screen.getByText(/receiver otlp deve/)).toBeTruthy();
  });
});
