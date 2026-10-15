import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { request } from "@/lib/api";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { ConfigField, ConfigurationSection } from "./configuration-form";
import type { Integration } from "./integration-manager";
export type SNMPCredentials = {
  community?: string;
  username?: string;
  security_level?: string;
  auth_protocol?: string;
  auth_password?: string;
  privacy_protocol?: string;
  privacy_password?: string;
  context_name?: string;
};
export type NetworkTemplate = {
  id: string;
  name: string;
  supported: number;
  unsupported: number;
  builtin?: boolean;
  warnings: string[];
  items: { name: string; key: string; supported: boolean; reason?: string }[];
  discovery: {
    name: string;
    supported: boolean;
    reason?: string;
    items: { name: string; supported: boolean; reason?: string }[];
  }[];
};
export type NetworkCatalog = {
  templates: NetworkTemplate[];
  credentials_configured: Record<string, boolean>;
};
const selectClass = "h-9 w-full border bg-surface-raised px-3 text-sm";
export function NativeNetworkFields({
  draft,
  onChange,
  catalog,
}: {
  draft: Integration;
  onChange: (value: Integration) => void;
  catalog?: NetworkCatalog;
}) {
  const credential = draft.credentials;
  const keep = !!catalog?.credentials_configured?.[draft.id] && !credential;
  const secret = (key: keyof SNMPCredentials, value: string) =>
    onChange({ ...draft, credentials: { ...credential, [key]: value } });
  return (
    <>
      <ConfigField>
        IP ou DNS do equipamento
        <Input
          required
          maxLength={253}
          value={draft.target || ""}
          placeholder="192.0.2.10"
          onChange={(e) => onChange({ ...draft, target: e.target.value })}
        />
      </ConfigField>
      <ConfigField>
        Porta SNMP
        <Input
          type="number"
          min={1}
          max={65535}
          required
          value={draft.snmp_port || 161}
          onChange={(e) =>
            onChange({ ...draft, snmp_port: Number(e.target.value) })
          }
        />
      </ConfigField>
      <ConfigField>
        Versão SNMP
        <select
          className={selectClass}
          required
          value={draft.snmp_version || "2c"}
          onChange={(e) =>
            onChange({
              ...draft,
              snmp_version: e.target.value,
              credentials:
                e.target.value === "3"
                  ? {
                      username: "",
                      security_level: "authPriv",
                      auth_protocol: "SHA256",
                      privacy_protocol: "AES",
                      auth_password: "",
                      privacy_password: "",
                    }
                  : { community: "" },
            })
          }
        >
          <option value="1">SNMPv1</option>
          <option value="2c">SNMPv2c</option>
          <option value="3">SNMPv3</option>
        </select>
      </ConfigField>
      {keep ? (
        <div className="configuration-help">
          <p>Credenciais cadastradas. Deixe como está para mantê-las.</p>
          <Button
            type="button"
            variant="outline"
            onClick={() =>
              onChange({
                ...draft,
                credentials:
                  draft.snmp_version === "3"
                    ? {
                        username: "",
                        security_level: "authPriv",
                        auth_protocol: "SHA256",
                        privacy_protocol: "AES",
                      }
                    : { community: "" },
              })
            }
          >
            Alterar credenciais
          </Button>
        </div>
      ) : draft.snmp_version !== "3" ? (
        <ConfigField>
          Comunidade SNMP
          <Input
            type="password"
            autoComplete="new-password"
            required
            maxLength={255}
            value={credential?.community || ""}
            onChange={(e) => secret("community", e.target.value)}
          />
        </ConfigField>
      ) : (
        <>
          <ConfigField>
            Usuário SNMPv3
            <Input
              required
              maxLength={64}
              value={credential?.username || ""}
              onChange={(e) => secret("username", e.target.value)}
            />
          </ConfigField>
          <ConfigField>
            Nível de segurança
            <select
              className={selectClass}
              required
              value={credential?.security_level || "authPriv"}
              onChange={(e) => secret("security_level", e.target.value)}
            >
              <option value="noAuthNoPriv">
                Sem autenticação e sem criptografia
              </option>
              <option value="authNoPriv">
                Com autenticação e sem criptografia
              </option>
              <option value="authPriv">Com autenticação e criptografia</option>
            </select>
          </ConfigField>
          {credential?.security_level !== "noAuthNoPriv" && (
            <>
              <ConfigField>
                Protocolo de autenticação
                <select
                  className={selectClass}
                  required
                  value={credential?.auth_protocol || "SHA256"}
                  onChange={(e) => secret("auth_protocol", e.target.value)}
                >
                  <option>SHA</option>
                  <option>SHA256</option>
                  <option>SHA512</option>
                </select>
              </ConfigField>
              <ConfigField>
                Senha de autenticação
                <Input
                  type="password"
                  autoComplete="new-password"
                  required
                  minLength={8}
                  value={credential?.auth_password || ""}
                  onChange={(e) => secret("auth_password", e.target.value)}
                />
              </ConfigField>
            </>
          )}
          {(credential?.security_level || "authPriv") === "authPriv" && (
            <>
              <ConfigField>
                Protocolo de privacidade
                <select
                  className={selectClass}
                  required
                  value={credential?.privacy_protocol || "AES"}
                  onChange={(e) => secret("privacy_protocol", e.target.value)}
                >
                  <option value="AES">AES128</option>
                  <option value="AES256">AES256 (Blumenthal)</option>
                </select>
              </ConfigField>
              <ConfigField>
                Senha de privacidade
                <Input
                  type="password"
                  autoComplete="new-password"
                  required
                  minLength={8}
                  value={credential?.privacy_password || ""}
                  onChange={(e) => secret("privacy_password", e.target.value)}
                />
              </ConfigField>
            </>
          )}
          <ConfigField>
            Contexto SNMPv3
            <Input
              maxLength={128}
              value={credential?.context_name || ""}
              onChange={(e) => secret("context_name", e.target.value)}
            />
          </ConfigField>
        </>
      )}
      <p className="configuration-help">
        Coleta pelo Vetra Server. O equipamento deve permitir consultas SNMP a
        partir do servidor. As credenciais ficam criptografadas no banco e não
        são devolvidas pela API. Use HTTPS para proteger o cadastro durante o
        transporte.
      </p>
    </>
  );
}
export function NetworkTemplateFields({
  draft,
  onChange,
  catalog,
}: {
  draft: Integration;
  onChange: (value: Integration) => void;
  catalog?: NetworkCatalog;
}) {
  const templates = catalog?.templates ?? [];
  const selected = templates.find((t) => t.id === draft.template_id);
  return (
    <>
      <ConfigField>
        Intervalo de coleta (segundos)
        <Input
          type="number"
          required
          min={30}
          max={86400}
          value={draft.interval_seconds || 300}
          onChange={(e) =>
            onChange({ ...draft, interval_seconds: Number(e.target.value) })
          }
        />
      </ConfigField>
      <ConfigField>
        Tipo de equipamento
        <select
          className={selectClass}
          required
          value={draft.device_type || "generic"}
          onChange={(e) => onChange({ ...draft, device_type: e.target.value })}
        >
          {[
            ["generic", "Genérico"],
            ["switch", "Switch"],
            ["router", "Roteador"],
            ["firewall", "Firewall"],
            ["wireless", "Controladora ou access point"],
            ["ups", "UPS / Nobreak"],
            ["printer", "Impressora"],
            ["storage", "Storage"],
            ["server", "Servidor ou appliance"],
            ["sensor", "Sensor / IoT"],
          ].map(([value, label]) => (
            <option key={value} value={value}>
              {label}
            </option>
          ))}
        </select>
      </ConfigField>
      <ConfigField>
        Grupo de equipamentos
        <Input
          maxLength={128}
          value={draft.host_group || ""}
          placeholder="Datacenter principal"
          onChange={(e) => onChange({ ...draft, host_group: e.target.value })}
        />
      </ConfigField>
      <ConfigField>
        Template de monitoramento
        <select
          className={selectClass}
          required
          value={draft.template_id || "generic-snmp"}
          onChange={(e) => onChange({ ...draft, template_id: e.target.value })}
        >
          <option value="generic-snmp">
            SNMP padrão · Sistema e interfaces
          </option>
          {templates
            .filter((t) => !t.builtin)
            .map((t) => (
              <option key={t.id} value={t.id}>
                {t.name} · {t.supported} suportados / {t.unsupported}{" "}
                incompatíveis
              </option>
            ))}
        </select>
      </ConfigField>
      {selected && !selected.builtin && (
        <p className="configuration-help">
          O perfil lê sistema e interfaces, mais os itens compatíveis deste
          template.{" "}
          {selected.unsupported > 0
            ? `${selected.unsupported} componentes não serão executados. Confira o relatório em Templates de rede.`
            : "Confira os avisos de compatibilidade no catálogo."}
        </p>
      )}
    </>
  );
}
export function NetworkTemplatesPanel({ demo }: { demo: boolean }) {
  const client = useQueryClient();
  const query = useQuery({
    queryKey: ["network-templates"],
    queryFn: () => request<NetworkCatalog>("/api/v1/network/templates"),
    enabled: !demo,
  });
  const [deleting, setDeleting] = useState("");
  async function remove(id: string) {
    setBusy(true);
    try {
      await request(`/api/v1/network/templates/${encodeURIComponent(id)}`, {
        method: "DELETE",
      });
      setDeleting("");
      setMessage("Template excluído.");
      await client.invalidateQueries({ queryKey: ["network-templates"] });
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Falha ao excluir");
    } finally {
      setBusy(false);
    }
  }
  const [document, setDocument] = useState("");
  const [report, setReport] = useState<NetworkTemplate[]>([]);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  async function run(preview: boolean) {
    setBusy(true);
    setMessage("");
    try {
      const result = await request<{ templates: NetworkTemplate[] }>(
        "/api/v1/network/templates/import",
        { method: "POST", body: JSON.stringify({ document, preview }) },
      );
      setReport(result.templates);
      setConfirmed(true);
      if (!preview) {
        setDocument("");
        setReport([]);
        setConfirmed(false);
        setMessage(
          "Templates importados. Selecione o template no cadastro do equipamento.",
        );
        await client.invalidateQueries({ queryKey: ["network-templates"] });
      }
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Falha ao importar");
      setConfirmed(false);
    } finally {
      setBusy(false);
    }
  }
  return (
    <details className="border border-border bg-surface p-4">
      <summary className="cursor-pointer text-sm font-semibold text-primary">
        Templates de rede · Importar do Zabbix
      </summary>
      <ConfigurationSection className="mt-4 space-y-4">
        <p className="text-sm text-muted-foreground">
          Importe YAML, JSON ou XML do Zabbix e revise a compatibilidade antes
          de salvar. Scripts, triggers e itens incompatíveis não são executados.
        </p>
        <ConfigField>
          Arquivo de template
          <input
            type="file"
            accept=".yaml,.yml,.json,.xml"
            disabled={demo || busy}
            onChange={async (e) => {
              setReport([]);
              setConfirmed(false);
              const file = e.target.files?.[0];
              if (!file) return;
              if (file.size > 2 * 1024 * 1024) {
                setDocument("");
                setMessage("Arquivo maior que 2 MiB.");
                return;
              }
              setDocument(await file.text());
              setMessage("");
            }}
          />
        </ConfigField>
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={demo || busy || !document}
            onClick={() => void run(true)}
          >
            Analisar compatibilidade
          </Button>
          <Button
            type="button"
            disabled={demo || busy || !document || !confirmed}
            onClick={() => void run(false)}
          >
            Importar templates analisados
          </Button>
        </div>
        {message && (
          <p role="status" className="text-sm">
            {message}
          </p>
        )}
        {query.error && <p role="alert">{query.error.message}</p>}
        {(report.length ? report : (query.data?.templates ?? [])).map((t) => (
          <section key={t.id} className="border border-border p-3">
            <h3 className="text-sm font-semibold">{t.name}</h3>
            {!t.builtin && !report.length && (
              <div className="mt-2 flex gap-2">
                {deleting === t.id ? (
                  <>
                    <Button
                      type="button"
                      variant="destructive"
                      disabled={demo || busy}
                      onClick={() => void remove(t.id)}
                    >
                      Confirmar exclusão
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      disabled={busy}
                      onClick={() => setDeleting("")}
                    >
                      Cancelar exclusão
                    </Button>
                  </>
                ) : (
                  <Button
                    type="button"
                    variant="outline"
                    disabled={demo || busy}
                    onClick={() => setDeleting(t.id)}
                  >
                    Excluir template
                  </Button>
                )}
              </div>
            )}
            <p className="text-xs text-muted-foreground">
              {t.builtin
                ? "Perfil nativo IF-MIB / IF-X-MIB"
                : `${t.supported} itens/protótipos compatíveis · ${t.unsupported} incompatíveis`}
            </p>
            {t.warnings?.map((warning, i) => (
              <p className="mt-2 text-xs text-warning" key={i}>
                {warning}
              </p>
            ))}
            {!t.builtin && (
              <details className="mt-3">
                <summary className="cursor-pointer text-xs">
                  Ver itens e descobertas
                </summary>
                <ul className="mt-2 max-h-64 overflow-y-auto space-y-2 text-xs">
                  {t.items?.map((item, i) => (
                    <li key={i}>
                      <strong>
                        {item.supported ? "Compatível" : "Não executado"}
                      </strong>{" "}
                      · {item.name} {item.reason && `— ${item.reason}`}
                    </li>
                  ))}
                  {t.discovery?.map((d, i) => (
                    <li key={`d${i}`}>
                      <strong>
                        {d.supported
                          ? "Descoberta compatível"
                          : "Descoberta não executada"}
                      </strong>{" "}
                      · {d.name} {d.reason && `— ${d.reason}`}
                      <ul>
                        {d.items
                          ?.filter((item) => !item.supported)
                          .map((item, j) => (
                            <li key={j}>
                              {item.name} — {item.reason}
                            </li>
                          ))}
                      </ul>
                    </li>
                  ))}
                </ul>
              </details>
            )}
          </section>
        ))}
      </ConfigurationSection>
    </details>
  );
}
