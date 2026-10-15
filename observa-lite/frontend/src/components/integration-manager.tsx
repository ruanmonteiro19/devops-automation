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
  azure: "AZURE — RESOURCE MANAGER",
  gcp: "GCP — COMPUTE ENGINE",
  snmp_exporter: "NETWORK DEVICE — SNMP EXPORTER",
};
const blank = (devices: boolean): Integration => ({
  id: crypto.randomUUID(),
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
    2: "DOWN",
    3: "TESTING",
    4: "UNKNOWN",
    5: "DORMANT",
    6: "NOT PRESENT",
    7: "LOWER LAYER DOWN",
  };
  const speed =
    (metrics["ifHighSpeed"] ?? 0) * 1000000 || (metrics["ifSpeed"] ?? 0);
  const format = (n: number | undefined, suffix: string) =>
    n === undefined
      ? "SEM AMOSTRA"
      : `${n.toLocaleString("pt-BR", { maximumFractionDigits: 2 })} ${suffix}`;
  const items = [
    [
      "ESTADO OPERACIONAL",
      metrics["ifOperStatus"] === undefined
        ? "NÃO RECEBIDO"
        : (oper[metrics["ifOperStatus"]!] ?? String(metrics["ifOperStatus"])),
    ],
    [
      "VELOCIDADE",
      speed > 0 ? format(speed / 1000000, "MBPS") : "NÃO RECEBIDA",
    ],
    [
      "TRÁFEGO DE ENTRADA",
      metrics["observa_in_bits_per_second"] === undefined
        ? "AGUARDANDO DUAS COLETAS"
        : format(metrics["observa_in_bits_per_second"]! / 1000000, "MBPS"),
    ],
    [
      "TRÁFEGO DE SAÍDA",
      metrics["observa_out_bits_per_second"] === undefined
        ? "AGUARDANDO DUAS COLETAS"
        : format(metrics["observa_out_bits_per_second"]! / 1000000, "MBPS"),
    ],
    [
      "UTILIZAÇÃO DE ENTRADA",
      format(metrics["observa_in_utilization_percent"], "%"),
    ],
    [
      "UTILIZAÇÃO DE SAÍDA",
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
          <p className="mb-2 text-xs text-primary">INTEGRAÇÕES</p>
          <h1 className="text-2xl font-semibold">
            {devices ? "NETWORK DEVICES" : "CLOUD PROVIDERS"}
          </h1>
          <p className="mt-2 max-w-3xl text-sm text-muted-foreground">
            {devices
              ? "DADOS DE SISTEMA E INTERFACES VIA PROMETHEUS SNMP EXPORTER. OS MÓDULOS E A AUTENTICAÇÃO SNMP SÃO CONFIGURADOS NO EXPORTER."
              : "INVENTÁRIO DE RECURSOS VIA API DOS PROVEDORES. CREDENCIAIS SOMENTE NO AMBIENTE DO SERVIDOR."}
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
          {devices ? "ADICIONAR EQUIPAMENTO" : "CONECTAR CLOUD"}
        </Button>
      </header>
      {demo && (
        <p className="border p-4 text-xs">
          DEMONSTRAÇÃO: CONEXÕES EXTERNAS E GRAVAÇÃO DESATIVADAS.
        </p>
      )}
      {query.isLoading && <p role="status">CARREGANDO INTEGRAÇÕES…</p>}
      {query.error && (
        <p role="alert" className="text-danger">
          {query.error.message}
          <Button variant="outline" onClick={() => query.refetch()}>
            TENTAR NOVAMENTE
          </Button>
        </p>
      )}
      {collect.isSuccess && (
        <p
          role="status"
          className="border border-primary bg-surface p-3 text-xs"
        >
          COLETA INICIADA. A ATUALIZAÇÃO DO RESULTADO É AUTOMÁTICA.
        </p>
      )}
      {collect.error && (
        <p role="alert" className="text-danger">
          {collect.error.message}
        </p>
      )}
      <Input
        aria-label="BUSCAR RECURSOS"
        placeholder="BUSCAR RECURSO, LOCALIZAÇÃO OU TIPO"
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
                  {labels[p.kind]} · {p.enabled ? "HABILITADO" : "PAUSADO"} ·{" "}
                  {query.data?.collecting?.includes(p.id)
                    ? "COLETANDO"
                    : result?.status || "SEM COLETA"}
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
                COLETAR AGORA
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
                EDITAR
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
                FALHA NA COLETA: {result.error}.{" "}
                {resources.length > 0 &&
                  "INVENTÁRIO ANTERIOR MANTIDO; DADOS DESATUALIZADOS."}
              </p>
            )}
            <div className="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3 text-xs text-muted-foreground">
              <p>
                {resources.length} RECURSOS · ÚLTIMA COLETA BEM-SUCEDIDA:{" "}
                {result?.success_at && !result.success_at.startsWith("0001")
                  ? new Date(result.success_at).toLocaleString("pt-BR")
                  : "NENHUMA"}
                {result?.truncated ? " · RESULTADO LIMITADO" : ""}
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
                EXPORTAR
              </Button>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[650px] text-left text-xs">
                <thead>
                  <tr>
                    {[
                      "RECURSO",
                      "TIPO",
                      "LOCALIZAÇÃO",
                      "ESTADO DO PROVEDOR",
                      "DETALHES",
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
                          ABRIR
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {!resources.length && (
                <p className="p-6 text-center text-xs text-muted-foreground">
                  SEM RECURSOS NESTA SELEÇÃO. CONFIRA A CONFIGURAÇÃO E O
                  RESULTADO DA COLETA.
                </p>
              )}
            </div>
          </section>
        );
      })}
      {!profiles.length && !query.isLoading && (
        <div className="border border-dashed p-8 text-center text-sm text-muted-foreground">
          NENHUMA INTEGRAÇÃO CONFIGURADA.
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
              {devices ? "CONFIGURAR EQUIPAMENTO" : "CONFIGURAR CLOUD"}
            </DialogTitle>
            <DialogDescription>
              PERFIL DE COLETA SOMENTE LEITURA. A COLETA AUTOMÁTICA OCORRE A
              CADA 5 MINUTOS.
            </DialogDescription>
          </DialogHeader>
          {draft && (
            <form onSubmit={submit} className="space-y-4">
              <label className="block text-sm">
                NOME
                <Input
                  required
                  maxLength={200}
                  value={draft.name}
                  onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                />
              </label>
              {!devices && (
                <label className="block text-sm">
                  PROVEDOR
                  <select
                    aria-label="PROVEDOR CLOUD"
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
                  REGIÃO AWS
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
                  {draft.kind === "azure" ? "SUBSCRIPTION ID" : "PROJECT ID"}
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
                  PREFIXO DAS VARIÁVEIS DE CREDENCIAL
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
                    URL BASE DO SNMP EXPORTER
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
                    IP DO EQUIPAMENTO
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
                    MÓDULOS DO EXPORTER
                    <Input
                      required
                      value={draft.module || ""}
                      onChange={(e) =>
                        setDraft({ ...draft, module: e.target.value })
                      }
                    />
                  </label>
                  <label className="block text-sm">
                    PERFIL AUTH DO EXPORTER
                    <Input
                      required
                      value={draft.auth || ""}
                      onChange={(e) =>
                        setDraft({ ...draft, auth: e.target.value })
                      }
                    />
                  </label>
                  <p className="text-xs text-muted-foreground">
                    PARA ENDEREÇOS INTERNOS, HABILITE
                    OBSERVA_ALLOW_PRIVATE_TARGETS NO SERVIDOR. MÉTRICAS
                    DISPONÍVEIS DEPENDEM DOS MIBS E MÓDULOS CONFIGURADOS; NÃO HÁ
                    DESCOBERTA FÍSICA LLDP/CDP NESTA VERSÃO.
                  </p>
                </>
              )}
              <div className="border bg-background p-4">
                <p className="mb-3 text-xs font-semibold">
                  CONFIGURAR NO AMBIENTE DO SERVIDOR
                </p>
                <pre
                  className="overflow-x-auto text-xs normal-case"
                  style={{ textTransform: "none" }}
                >
                  {envHelp}
                </pre>
                <p className="mt-3 text-xs text-muted-foreground">
                  NÃO COLE CREDENCIAIS NESTE FORMULÁRIO. CONSULTE O GUIA DE
                  INTEGRAÇÕES DO PROJETO PARA INJETAR AS VARIÁVEIS NO CONTAINER.
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
                COLETA HABILITADA
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
                  CANCELAR
                </Button>
                <Button disabled={save.isPending || demo}>
                  {save.isPending ? "SALVANDO…" : "SALVAR PERFIL"}
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
            <DialogTitle>EXCLUIR INTEGRAÇÃO?</DialogTitle>
            <DialogDescription>
              O PERFIL E O INVENTÁRIO LOCAL SERÃO REMOVIDOS. NENHUM RECURSO
              EXTERNO SERÁ ALTERADO.
            </DialogDescription>
          </DialogHeader>
          {remove.error && <p role="alert">{remove.error.message}</p>}
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              disabled={remove.isPending}
              onClick={() => setDeleting(null)}
            >
              CANCELAR
            </Button>
            <Button
              variant="destructive"
              disabled={remove.isPending || demo}
              onClick={() => {
                if (deleting) remove.mutate(deleting.id);
              }}
            >
              EXCLUIR
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
              INVENTÁRIO OBSERVADO · NÃO REPRESENTA SAÚDE INFERIDA
            </SheetDescription>
          </SheetHeader>
          {selected && (
            <div className="space-y-5 p-5">
              <p className="break-all text-xs text-muted-foreground">
                {selected.resource.id}
              </p>
              {selected.result.status === "error" && (
                <p className="text-xs text-warning">
                  DADOS DA ÚLTIMA COLETA BEM-SUCEDIDA. A COLETA ATUAL FALHOU.
                </p>
              )}
              <h2 className="text-sm font-semibold">ATRIBUTOS</h2>
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
              <h2 className="text-sm font-semibold">MÉTRICAS DE SISTEMA</h2>
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
                    {iface.name} · IFINDEX {iface.index}
                  </h3>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {iface.description}
                  </p>
                  <InterfaceSummary metrics={iface.metrics} />
                  <details>
                    <summary className="cursor-pointer text-xs text-primary">
                      MÉTRICAS ORIGINAIS DA INTERFACE
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
                CONTADORES SÃO ACUMULADOS. OBSERVA_* INDICA TAXAS E UTILIZAÇÃO
                APÓS DUAS COLETAS VÁLIDAS. CPU, MEMÓRIA E OUTROS INDICADORES SÓ
                APARECEM SE PUBLICADOS PELO MÓDULO.
              </p>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
