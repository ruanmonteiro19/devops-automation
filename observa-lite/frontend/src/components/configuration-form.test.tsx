// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import {
  ConfigField,
  ConfigurationForm,
  ConfigurationTabs,
} from "./configuration-form";
afterEach(cleanup);
describe("Configuration forms", () => {
  it("associates labels and exposes native requirements without changing field names", () => {
    render(
      <ConfigurationForm>
        <ConfigField>
          Nome
          <input required defaultValue="Switch" />
        </ConfigField>
        <ConfigField>
          Descrição
          <textarea />
        </ConfigField>
        <ConfigField>
          <input type="checkbox" />
          Habilitado
        </ConfigField>
      </ConfigurationForm>,
    );
    expect((screen.getByLabelText("Nome") as HTMLInputElement).required).toBe(
      true,
    );
    expect(
      (screen.getByLabelText("Descrição") as HTMLTextAreaElement).required,
    ).toBe(false);
    expect(screen.getAllByText("* Obrigatório")).toHaveLength(1);
    expect(screen.getAllByText("Opcional")).toHaveLength(1);
    expect(screen.getByLabelText("Habilitado").getAttribute("type")).toBe(
      "checkbox",
    );
  });
  it("keeps values mounted between tabs and supports arrow keys", () => {
    render(
      <ConfigurationForm>
        <ConfigurationTabs
          sections={[
            {
              id: "general",
              title: "Geral",
              content: (
                <ConfigField>
                  Nome
                  <input defaultValue="Switch" />
                </ConfigField>
              ),
            },
            {
              id: "connection",
              title: "Conexão",
              content: (
                <ConfigField>
                  Destino
                  <input required />
                </ConfigField>
              ),
            },
          ]}
        />
      </ConfigurationForm>,
    );
    fireEvent.change(screen.getByLabelText("Nome"), {
      target: { value: "Core switch" },
    });
    fireEvent.keyDown(screen.getByRole("tab", { name: "Geral" }), {
      key: "ArrowRight",
    });
    expect(
      screen
        .getByRole("tab", { name: "Conexão" })
        .getAttribute("aria-selected"),
    ).toBe("true");
    fireEvent.keyDown(screen.getByRole("tab", { name: "Conexão" }), {
      key: "Home",
    });
    expect((screen.getByLabelText("Nome") as HTMLInputElement).value).toBe(
      "Core switch",
    );
  });
  it("opens the hidden section containing an invalid required field", () => {
    vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
      callback(0);
      return 0;
    });
    render(
      <ConfigurationForm>
        <ConfigurationTabs
          sections={[
            { id: "general", title: "Geral", content: <p>Geral</p> },
            {
              id: "connection",
              title: "Conexão",
              content: (
                <ConfigField>
                  Destino
                  <input required />
                </ConfigField>
              ),
            },
          ]}
        />
      </ConfigurationForm>,
    );
    // The native reportValidity call is checked in the browser; avoid reentry in jsdom.
    const target = screen.getByLabelText("Destino") as HTMLInputElement;
    vi.spyOn(target, "reportValidity").mockReturnValue(false);
    fireEvent.invalid(target);
    expect(
      screen
        .getByRole("tab", { name: "Conexão" })
        .getAttribute("aria-selected"),
    ).toBe("true");
    vi.unstubAllGlobals();
  });
});
