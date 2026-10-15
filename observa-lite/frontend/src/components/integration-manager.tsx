import { newID } from "@/lib/id";
import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Cloud,
  Network,
  Plus,
  RefreshCw,
  Pencil,
  Trash2,
  ChevronRight,
  Download,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import { request } from "@/lib/api";
import { downloadJSON } from "@/lib/workspace";
export type Integration = {
  id: string;
  name: string;
  kind: string;
  enabled: boolean;
  region?: string;
  scope?: string;
  credential_prefix?: string;
  endpoint?: string;
  target?: string;
  module?: string;
  auth?: string;
};
export type Resource = {
  id: string;
  name: string;
  kind: string;
  location?: string;
  status: string;
  attributes?: Record<string, string>;
  metrics?: Record<string, number>;
  interfaces?: {
    index: string;
    name: string;
    description?: string;
    metrics: Record<string, number>;
  }[];
};
type Result = {
  at: string;
  success_at: string;
  status: string;
  error?: string;
  truncated: boolean;
  resources: Resource[];
};
export type IntegrationState = {
  collecting?: string[];
  integrations: Integration[];
  results: Record<string, Result>;
};
const labels: Record<string, string> = {
  aws: "AWS — EC2",
  azure: "Azure — resource manager",
  gcp: "GCP — Compute engine",
  snmp_exporter: "Network device — SNMP exporter",
};
const blank = (devices: boolean): Integration => ({
  id: newID(),
  name: "",
  kind: devices ? "snmp_exporter" : "aws",
  enabled: true,
  region: "us-east-1",
  scope: "",
  credential_prefix: "OBSERVA_CLOUD",
  endpoint: "",
  target: "",
  module: "if_mib",
  auth: "public_v2",
});
function InterfaceSummary({ metrics }: { metrics: Record<string, number> }) {
  const oper: Record<number, string> = {
    1: "UP",
    2: "Down",
    3: "Testing",
    4: "UNKNOWN",
    5: "Dormant",
    6: "Not present",
    7: "Lower layer down",
  };
  const speed =
    (metrics["ifHighSpeed"] ?? 0) * 1000000 || (metrics["ifSpeed"] ?? 0);
  const format = (n: number | undefined, suffix: string) =>
    n === undefined
      ? "Sem amostra"
      : `${n.toLocaleString("pt-BR", { maximumFractionDigits: 2 })} ${suffix}`;
  const items = [
    [
      "Estado operacional",
      metrics["ifOperStatus"] === undefined
        ? "Não recebido"
        : (oper[metrics["ifOperStatus"]!] ?? String(metrics["ifOperStatus"])),
    ],
    [
      "Velocidade",
      speed > 0 ? format(speed / 1000000, "Mbps") : "Não recebida",
    ],
    [
      "Tráfego de entrada",
      metrics["observa_in_bits_per_second"] === undefined
        ? "Aguardando duas coletas"
        : format(metrics["observa_in_bits_per_second"]! / 1000000, "Mbps"),
    ],
    [
      "Tráfego de saída",
      metrics["observa_out_bits_per_second"] === undefined
        ? "Aguardando duas coletas"
        : format(metrics["observa_out_bits_per_second"]! / 1000000, "Mbps"),
    ],
    [
      "Utilização de entrada",
      format(metrics["observa_in_utilization_percent"], "%"),
    ],
    [
      "Utilização de saída",
      format(metrics["observa_out_utilization_percent"], "%"),
    ],
  ];
  return (
    <dl className="my-4 grid grid-cols-2 gap-3 border bg-background p-3 text-xs">
      {items.map(([label, value]) => (
        <div key={label}>
          <dt className="text-muted-foreground">{label}</dt>
          <dd className="mt-1 font-medium">{value}</dd>
        </div>
      ))}
    </dl>
  );
}
export function IntegrationManager({
  demo,
  devices = false,
  initialCreate = false,
}: {
  demo: boolean;
  devices?: boolean;
  initialCreate?: boolean;
}) {
  const client = useQueryClient();
  const [draft, setDraft] = useState<Integration | null>(() =>
    initialCreate && !demo ? blank(devices) : null,
  );
  const [deleting, setDeleting] = useState<Integration | null>(null);
  const [selected, setSelected] = useState<{
    resource: Resource;
    result: Result;
  } | null>(null);
  const [search, setSearch] = useState("");
  const query = useQuery({
    queryKey: ["integrations"],
    queryFn: () => request<IntegrationState>("/api/v1/integrations"),
    enabled: !demo,
    refetchInterval: 5000,
    retry: false,
  });
  const invalidate = () => {
    void client.invalidateQueries({ queryKey: ["integrations"] });
    void client.invalidateQueries({ queryKey: ["topology"] });
  };
  const save = useMutation({
    mutationFn: (p: Integration) =>
      request("/api/v1/integrations", {
        method: "POST",
        body: JSON.stringify(p),
      }),
    onSuccess: () => {
      setDraft(null);
      invalidate();
    },
  });
  const remove = useMutation({
    mutationFn: (id: string) =>
      request(`/api/v1/integrations/${encodeURIComponent(id)}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      setDeleting(null);
      invalidate();
    },
  });
  const collect = useMutation({
    mutationFn: (id: string) =>
      request(`/api/v1/integrations/${encodeURIComponent(id)}/collect`, {
        method: "POST",
      }),
    onSuccess: invalidate,
  });
  function submit(event: FormEvent) {
    event.preventDefault();
    if (!demo && draft) save.mutate(draft);
  }
  const profiles = (query.data?.integrations ?? []).filter(
    (p) => (p.kind === "snmp_exporter") === devices,
  );
  const prefix = draft?.credential_prefix || "OBSERVA_CLOUD";
  const envHelp =
    draft?.kind === "aws"
      ? `${prefix}_ACCESS_KEY_ID=...\n${prefix}_SECRET_ACCESS_KEY=...\n# Opcional para credenciais temporárias:\n${prefix}_SESSION_TOKEN=...`
      : draft?.kind === "azure"
        ? `${prefix}_TENANT_ID=...\n${prefix}_CLIENT_ID=...\n${prefix}_CLIENT_SECRET=...`
        : draft?.kind === "gcp"
          ? `${prefix}_ACCESS_TOKEN=...\n# Token OAuth com Compute Viewer/read-only; renovar antes de expirar.`
          : "# Community/SNMPv3 configurados no snmp.yml do exporter.\n# Aqui informe apenas o nome do perfil auth, nunca o segredo.";
  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="mb-2 text-xs text-primary">Integrações</p>
          <h1 className="text-2xl font-semibold">
            {devices ? "Network devices" : "Cloud providers"}
          </h1>
          <p className="mt-2 max-w-3xl text-sm text-muted-foreground">
            {devices
              ? "Dados de sistema e interfaces via prometheus SNMP exporter. Os módulos e a autenticação SNMP são configurados no exporter."
              : "Inventário de recursos via API dos provedores. Credenciais somente no ambiente do servidor."}
          </p>
        </div>
        <Button
          disabled={demo}
          onClick={() => {
            save.reset();
            setDraft(blank(devices));
          }}
        >
          <Plus className="size-4" />
          {devices ? "Adicionar equipamento" : "Conectar cloud"}
        </Button>
      </header>
      {demo && (
        <p className="border p-4 text-xs">
          Demonstração: conexões externas e gravação desativadas.
        </p>
      )}
      {query.isLoading && <p role="status">Carregando integrações…</p>}
      {query.error && (
        <p role="alert" className="text-danger">
          {query.error.message}
          <Button variant="outline" onClick={() => query.refetch()}>
            Tentar novamente
          </Button>
        </p>
      )}
      {collect.isSuccess && (
        <p
          role="status"
          className="border border-primary bg-surface p-3 text-xs"
        >
          Coleta iniciada. A atualização do resultado é automática.
        </p>
      )}
      {collect.error && (
        <p role="alert" className="text-danger">
          {collect.error.message}
        </p>
      )}
      <Input
        aria-label="Buscar recursos"
        placeholder="Buscar recurso, localização ou tipo"
        className="max-w-md"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      {profiles.map((p) => {
        const result = query.data?.results[p.id];
        const resources = (result?.resources ?? []).filter((v) =>
          `${v.name} ${v.kind} ${v.location ?? ""} ${v.id}`
            .toLowerCase()
            .includes(search.toLowerCase()),
        );
        return (
          <section key={p.id} className="overflow-hidden border bg-surface">
            <header className="flex flex-wrap items-center gap-3 border-b bg-surface-raised p-4">
              {devices ? (
                <Network className="size-5 text-primary" />
              ) : (
                <Cloud className="size-5 text-primary" />
              )}
              <div className="mr-auto">
                <h2 className="font-semibold">{p.name}</h2>
                <p className="mt-1 text-[10px] text-muted-foreground">
                  {labels[p.kind]} · {p.enabled ? "Habilitado" : "Pausado"} ·{" "}
                  {query.data?.collecting?.includes(p.id)
                    ? "Coletando"
                    : result?.status || "Sem coleta"}
                </p>
              </div>
              <Button
                variant="outline"
                size="sm"
                disabled={
                  !p.enabled ||
                  collect.isPending ||
                  demo ||
                  query.data?.collecting?.includes(p.id)
                }
                onClick={() => collect.mutate(p.id)}
              >
                <RefreshCw className="size-3.5" />
                Coletar agora
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={demo}
                onClick={() => {
                  save.reset();
                  setDraft({ ...p });
                }}
              >
                <Pencil className="size-3.5" />
                Editar
              </Button>
              <Button
                variant="ghost"
                size="sm"
                aria-label={`EXCLUIR ${p.name}`}
                disabled={demo}
                onClick={() => {
                  remove.reset();
                  setDeleting(p);
                }}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </header>
            {result?.error && (
              <p
                role="alert"
                className="border-b bg-background p-4 text-xs text-danger"
              >
                Falha na coleta: {result.error}.{" "}
                {resources.length > 0 &&
                  "Inventário anterior mantido; dados desatualizados."}
              </p>
            )}
            <div className="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3 text-xs text-muted-foreground">
              <p>
                {resources.length} Recursos · última coleta bem-sucedida:{" "}
                {result?.success_at && !result.success_at.startsWith("0001")
                  ? new Date(result.success_at).toLocaleString("pt-BR")
                  : "Nenhuma"}
                {result?.truncated ? " · Resultado limitado" : ""}
              </p>
              <Button
                variant="ghost"
                size="sm"
                onClick={() =>
                  downloadJSON("observa-inventario.json", {
                    profile: p,
                    result,
                  })
                }
              >
                <Download className="size-3.5" />
                Exportar
              </Button>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[650px] text-left text-xs">
                <thead>
                  <tr>
                    {[
                      "Recurso",
                      "Tipo",
                      "Localização",
                      "Estado do provedor",
                      "Detalhes",
                    ].map((label) => (
                      <th key={label} className="px-4 py-3">
                        {label}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {resources.map((v) => (
                    <tr key={v.id} className="border-t">
                      <td className="px-4 py-3 font-medium">{v.name}</td>
                      <td className="px-4">{v.kind}</td>
                      <td className="px-4">{v.location || p.target || "—"}</td>
                      <td className="px-4">{v.status}</td>
                      <td className="px-4">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() =>
                            setSelected({ resource: v, result: result! })
                          }
                        >
                          <ChevronRight className="size-4" />
                          Abrir
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {!resources.length && (
                <p className="p-6 text-center text-xs text-muted-foreground">
                  Sem recursos nesta seleção. confira a configuração e o
                  resultado da coleta.
                </p>
              )}
            </div>
          </section>
        );
      })}
      {!profiles.length && !query.isLoading && (
        <div className="border border-dashed p-8 text-center text-sm text-muted-foreground">
          Nenhuma integração configurada.
        </div>
      )}
      <Dialog
        open={draft !== null}
        onOpenChange={(open) => {
          if (!open && !save.isPending) setDraft(null);
        }}
      >
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {devices ? "Configurar equipamento" : "Configurar cloud"}
            </DialogTitle>
            <DialogDescription>
              Perfil de coleta somente leitura. A coleta automática ocorre a
              cada 5 minutos.
            </DialogDescription>
          </DialogHeader>
          {draft && (
            <form onSubmit={submit} className="space-y-4">
              <label className="block text-sm">
                Nome
                <Input
                  required
                  maxLength={200}
                  value={draft.name}
                  onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                />
              </label>
              {!devices && (
                <label className="block text-sm">
                  Provedor
                  <select
                    aria-label="Provedor cloud"
                    className="mt-2 h-10 w-full border bg-background px-3"
                    value={draft.kind}
                    onChange={(e) =>
                      setDraft({ ...draft, kind: e.target.value })
                    }
                  >
                    {Object.entries(labels)
                      .filter(([key]) => key !== "snmp_exporter")
                      .map(([key, label]) => (
                        <option key={key} value={key}>
                          {label}
                        </option>
                      ))}
                  </select>
                </label>
              )}
              {draft.kind === "aws" && (
                <label className="block text-sm">
                  Região AWS
                  <Input
                    required
                    value={draft.region || ""}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        region: e.target.value.toLowerCase(),
                      })
                    }
                  />
                </label>
              )}
              {["azure", "gcp"].includes(draft.kind) && (
                <label className="block text-sm">
                  {draft.kind === "azure" ? "Subscription ID" : "Project ID"}
                  <Input
                    required
                    value={draft.scope || ""}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        scope:
                          draft.kind === "gcp"
                            ? e.target.value.toLowerCase()
                            : e.target.value,
                      })
                    }
                  />
                </label>
              )}
              {!devices && (
                <label className="block text-sm">
                  Prefixo das variáveis de credencial
                  <Input
                    required
                    pattern="OBSERVA_[A-Z][A-Z0-9_]*"
                    value={draft.credential_prefix || ""}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        credential_prefix: e.target.value.toUpperCase(),
                      })
                    }
                  />
                </label>
              )}
              {devices && (
                <>
                  <label className="block text-sm">
                    URL Base do SNMP exporter
                    <Input
                      type="url"
                      required
                      placeholder="http://snmp-exporter:9116"
                      value={draft.endpoint || ""}
                      onChange={(e) =>
                        setDraft({ ...draft, endpoint: e.target.value })
                      }
                    />
                  </label>
                  <label className="block text-sm">
                    IP Do equipamento
                    <Input
                      required
                      placeholder="192.0.2.10"
                      value={draft.target || ""}
                      onChange={(e) =>
                        setDraft({ ...draft, target: e.target.value })
                      }
                    />
                  </label>
                  <label className="block text-sm">
                    Módulos do exporter
                    <Input
                      required
                      value={draft.module || ""}
                      onChange={(e) =>
                        setDraft({ ...draft, module: e.target.value })
                      }
                    />
                  </label>
                  <label className="block text-sm">
                    Perfil auth do exporter
                    <Input
                      required
                      value={draft.auth || ""}
                      onChange={(e) =>
                        setDraft({ ...draft, auth: e.target.value })
                      }
                    />
                  </label>
                  <p className="text-xs text-muted-foreground">
                    Para endereços internos, habilite
                    observa_allow_private_targets no servidor. Métricas
                    disponíveis dependem dos mibs e módulos configurados; não há
                    descoberta física lldp/cdp nesta versão.
                  </p>
                </>
              )}
              <div className="border bg-background p-4">
                <p className="mb-3 text-xs font-semibold">
                  Configurar no ambiente do servidor
                </p>
                <pre
                  className="overflow-x-auto text-xs normal-case"
                  style={{ textTransform: "none" }}
                >
                  {envHelp}
                </pre>
                <p className="mt-3 text-xs text-muted-foreground">
                  Não cole credenciais neste formulário. consulte o guia de
                  integrações do projeto para injetar as variáveis no container.
                </p>
              </div>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={draft.enabled}
                  onChange={(e) =>
                    setDraft({ ...draft, enabled: e.target.checked })
                  }
                />
                Coleta habilitada
              </label>
              {save.error && (
                <p role="alert" className="text-danger">
                  {save.error.message}
                </p>
              )}
              <div className="flex justify-end gap-2">
                <Button
                  type="button"
                  variant="outline"
                  disabled={save.isPending}
                  onClick={() => setDraft(null)}
                >
                  Cancelar
                </Button>
                <Button disabled={save.isPending || demo}>
                  {save.isPending ? "Salvando…" : "Salvar perfil"}
                </Button>
              </div>
            </form>
          )}
        </DialogContent>
      </Dialog>
      <Dialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open && !remove.isPending) setDeleting(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Excluir integração?</DialogTitle>
            <DialogDescription>
              O perfil e o inventário local serão removidos. nenhum recurso
              externo será alterado.
            </DialogDescription>
          </DialogHeader>
          {remove.error && <p role="alert">{remove.error.message}</p>}
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              disabled={remove.isPending}
              onClick={() => setDeleting(null)}
            >
              Cancelar
            </Button>
            <Button
              variant="destructive"
              disabled={remove.isPending || demo}
              onClick={() => {
                if (deleting) remove.mutate(deleting.id);
              }}
            >
              Excluir
            </Button>
          </div>
        </DialogContent>
      </Dialog>
      <Sheet
        open={selected !== null}
        onOpenChange={(open) => {
          if (!open) setSelected(null);
        }}
      >
        <SheetContent className="overflow-y-auto sm:max-w-2xl">
          <SheetHeader>
            <SheetTitle>{selected?.resource.name}</SheetTitle>
            <SheetDescription>
              Inventário observado · não representa saúde inferida
            </SheetDescription>
          </SheetHeader>
          {selected && (
            <div className="space-y-5 p-5">
              <p className="break-all text-xs text-muted-foreground">
                {selected.resource.id}
              </p>
              {selected.result.status === "error" && (
                <p className="text-xs text-warning">
                  Dados da última coleta bem-sucedida. A coleta atual falhou.
                </p>
              )}
              <h2 className="text-sm font-semibold">Atributos</h2>
              <dl className="space-y-2">
                {Object.entries(selected.resource.attributes ?? {}).map(
                  ([key, value]) => (
                    <div key={key} className="border p-3 text-xs">
                      <dt className="text-muted-foreground">{key}</dt>
                      <dd className="mt-1 break-all">{value}</dd>
                    </div>
                  ),
                )}
              </dl>
              <h2 className="text-sm font-semibold">Métricas de sistema</h2>
              {Object.entries(selected.resource.metrics ?? {}).map(
                ([key, value]) => (
                  <div
                    key={key}
                    className="flex justify-between gap-3 border-b py-2 text-xs"
                  >
                    <span>{key}</span>
                    <span>{value.toLocaleString("pt-BR")}</span>
                  </div>
                ),
              )}
              {selected.resource.interfaces?.map((iface) => (
                <section key={iface.index} className="border p-4">
                  <h3 className="text-sm font-semibold">
                    {iface.name} · IfIndex {iface.index}
                  </h3>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {iface.description}
                  </p>
                  <InterfaceSummary metrics={iface.metrics} />
                  <details>
                    <summary className="cursor-pointer text-xs text-primary">
                      Métricas originais da interface
                    </summary>
                    <dl className="mt-3 grid grid-cols-2 gap-3 text-xs">
                      {Object.entries(iface.metrics).map(([key, value]) => (
                        <div key={key}>
                          <dt className="break-all text-muted-foreground">
                            {key}
                          </dt>
                          <dd className="mt-1">
                            {value.toLocaleString("pt-BR")}
                          </dd>
                        </div>
                      ))}
                    </dl>
                  </details>
                </section>
              ))}
              <p className="text-xs text-muted-foreground">
                Contadores são acumulados. Observa_* indica taxas e utilização
                após duas coletas válidas. CPU, memória e outros indicadores só
                aparecem se publicados pelo módulo.
              </p>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
