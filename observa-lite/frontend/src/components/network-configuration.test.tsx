// @vitest-environment jsdom
import { useState } from "react";
import { afterEach, describe, it, expect } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ConfigurationForm } from "./configuration-form";
import { NativeNetworkFields } from "./network-configuration";
import type { Integration } from "./integration-manager";
afterEach(cleanup);
function Editor({ configured = false }: { configured?: boolean }) {
  const [draft, setDraft] = useState<Integration>({
    id: "switch",
    name: "Core",
    kind: "snmp",
    enabled: true,
    snmp_version: "2c",
    target: "10.0.0.1",
    template_id: "generic-snmp",
    ...(configured ? {} : { credentials: { community: "" } }),
  });
  return (
    <ConfigurationForm>
      <NativeNetworkFields
        draft={draft}
        onChange={setDraft}
        catalog={{
          templates: [],
          credentials_configured: { switch: configured },
        }}
      />
    </ConfigurationForm>
  );
}
describe("Native network credentials", () => {
  it("keeps a stored secret until the operator explicitly replaces it", () => {
    render(<Editor configured />);
    expect(screen.queryByLabelText("Comunidade SNMP")).toBeNull();
    fireEvent.click(
      screen.getByRole("button", { name: "Alterar credenciais" }),
    );
    const field = screen.getByLabelText("Comunidade SNMP") as HTMLInputElement;
    expect(field.required).toBe(true);
    expect(field.value).toBe("");
    expect(field.type).toBe("password");
  });
  it("shows only the credentials required by the selected SNMPv3 security level", () => {
    render(<Editor />);
    fireEvent.change(screen.getByLabelText("Versão SNMP"), {
      target: { value: "3" },
    });
    expect(screen.queryByLabelText("Comunidade SNMP")).toBeNull();
    expect(
      (screen.getByLabelText("Senha de privacidade") as HTMLInputElement)
        .required,
    ).toBe(true);
    fireEvent.change(screen.getByLabelText("Nível de segurança"), {
      target: { value: "noAuthNoPriv" },
    });
    expect(screen.queryByLabelText("Senha de autenticação")).toBeNull();
    expect(screen.queryByLabelText("Senha de privacidade")).toBeNull();
    expect(
      (screen.getByLabelText("Usuário SNMPv3") as HTMLInputElement).required,
    ).toBe(true);
  });
});
