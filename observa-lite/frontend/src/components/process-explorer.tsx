import { useState } from "react";
import { Link } from "@tanstack/react-router";
import {
  Search,
  Download,
  Activity,
  Server,
  Settings2,
  ArrowUpDown,
  CircleHelp,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import { type Snapshot } from "@/lib/api";
import { observedProcesses, type ObservedProcess } from "@/lib/processes";
import { downloadJSON } from "@/lib/workspace";
const memory = (n: number) =>
  `${(n / 1048576).toLocaleString("pt-BR", { maximumFractionDigits: 1 })} MB`;
export function ProcessExplorer({ snapshot }: { snapshot: Snapshot }) {
  const all = observedProcesses(snapshot.hosts);
  const [search, setSearch] = useState("");
  const [host, setHost] = useState("");
  const [runtime, setRuntime] = useState("");
  const [showSystem, setShowSystem] = useState(false);
  const [sort, setSort] = useState("memory");
  const [limit, setLimit] = useState(100);
  const [selected, setSelected] = useState<ObservedProcess | null>(null);
  const filtered = all
    .filter(
      (p) =>
        (showSystem || !p.system) &&
        (!host || p.host_id === host) &&
        (!runtime || p.runtime === runtime) &&
        `${p.name} ${p.service} ${p.runtime} ${p.host_name} ${p.pid}`
          .toLowerCase()
          .includes(search.toLowerCase()),
    )
    .sort((a, b) =>
      sort === "memory"
        ? b.rss_bytes - a.rss_bytes || a.name.localeCompare(b.name)
        : sort === "pid"
          ? a.pid - b.pid
          : a.name.localeCompare(b.name),
    );
  const connections = selected
    ? (snapshot.hosts
        .find((h) => h.id === selected.host_id)
        ?.connections.filter((c) => c.pid === selected.pid) ?? [])
    : [];
  const systemCount = all.filter((p) => p.system).length;
  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="mb-2 text-xs font-semibold text-primary">
            INFRAESTRUTURA
          </p>
          <h1 className="text-2xl font-semibold">PROCESSOS DE APLICAÇÃO</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            PROCESSOS OBSERVADOS PELOS AGENTES. COMPONENTES DE SISTEMA OCULTOS
            POR PADRÃO.
          </p>
        </div>
        <Button
          variant="outline"
          onClick={() => downloadJSON("observa-processos.json", filtered)}
        >
          <Download className="size-4" />
          EXPORTAR RESULTADOS
        </Button>
      </header>
      <div className="grid gap-3 sm:grid-cols-3">
        {[
          ["APLICAÇÕES E OUTROS PROCESSOS", all.length - systemCount],
          ["HOSTS", snapshot.hosts.length],
          ["PROCESSOS DE SISTEMA OCULTOS", showSystem ? 0 : systemCount],
        ].map(([label, value]) => (
          <div key={label} className="border bg-surface p-4">
            <p className="text-xs text-muted-foreground">{label}</p>
            <p className="mt-2 text-2xl font-semibold">{value}</p>
          </div>
        ))}
      </div>
      <section className="flex flex-wrap items-center gap-3 border bg-surface p-3">
        <div className="relative min-w-52 flex-1">
          <Search className="absolute left-3 top-3 size-4 text-primary" />
          <Input
            aria-label="BUSCAR PROCESSOS"
            placeholder="BUSCAR PROCESSO, SERVIÇO OU PID"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
        <select
          aria-label="FILTRAR HOST"
          className="h-10 max-w-60 border bg-background px-3 text-xs"
          value={host}
          onChange={(e) => setHost(e.target.value)}
        >
          <option value="">TODOS OS HOSTS</option>
          {snapshot.hosts.map((h) => (
            <option key={h.id} value={h.id}>
              {h.name}
            </option>
          ))}
        </select>
        <select
          aria-label="FILTRAR RUNTIME"
          className="h-10 border bg-background px-3 text-xs"
          value={runtime}
          onChange={(e) => setRuntime(e.target.value)}
        >
          <option value="">TODOS OS RUNTIMES</option>
          {[...new Set(all.map((p) => p.runtime).filter(Boolean))]
            .sort()
            .map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
        </select>
        <select
          aria-label="ORDENAR PROCESSOS"
          className="h-10 border bg-background px-3 text-xs"
          value={sort}
          onChange={(e) => setSort(e.target.value)}
        >
          <option value="memory">MAIOR MEMÓRIA RSS</option>
          <option value="name">NOME</option>
          <option value="pid">PID</option>
        </select>
        <label className="flex items-center gap-2 border bg-background px-3 py-2.5 text-xs">
          <input
            type="checkbox"
            checked={showSystem}
            onChange={(e) => setShowSystem(e.target.checked)}
          />
          MOSTRAR PROCESSOS DE SISTEMA
        </label>
      </section>
      <div className="flex flex-wrap items-center justify-between gap-3 text-xs text-muted-foreground">
        <p>
          {filtered.length} RESULTADOS · {Math.min(limit, filtered.length)}{" "}
          EXIBIDOS
        </p>
        <div className="flex items-center gap-2">
          <span>EXIBIR</span>
          <select
            aria-label="LIMITE DE PROCESSOS"
            className="border bg-surface p-2"
            value={limit}
            onChange={(e) => setLimit(Number(e.target.value))}
          >
            <option value={100}>100</option>
            <option value={500}>500</option>
            <option value={1000}>1000</option>
          </select>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setSearch("");
              setHost("");
              setRuntime("");
              setShowSystem(false);
              setSort("memory");
            }}
          >
            LIMPAR FILTROS
          </Button>
        </div>
      </div>
      <div className="overflow-x-auto border bg-surface">
        <table className="w-full min-w-[850px] text-left text-xs">
          <thead className="bg-surface-raised">
            <tr>
              {[
                "PROCESSO",
                "PID",
                "SERVIÇO",
                "RUNTIME",
                "HOST",
                "MEMÓRIA RSS",
                "DEEP MONITORING",
                "AÇÕES",
              ].map((h) => (
                <th key={h} className="px-4 py-3 font-medium">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.slice(0, limit).map((p) => (
              <tr key={`${p.host_id}-${p.pid}`} className="border-t">
                <td className="px-4 py-3">
                  <button
                    className="flex items-center gap-2 text-left font-medium text-foreground hover:text-primary"
                    onClick={() => setSelected(p)}
                  >
                    <Activity className="size-4 shrink-0 text-primary" />
                    {p.name}
                  </button>
                  {p.system && (
                    <p className="mt-1 text-[10px] text-muted-foreground">
                      SISTEMA
                    </p>
                  )}
                </td>
                <td className="px-4">{p.pid}</td>
                <td className="px-4">{p.service || "—"}</td>
                <td className="px-4">{p.runtime || "NÃO IDENTIFICADO"}</td>
                <td className="px-4">{p.host_name}</td>
                <td className="px-4 tabular-nums">{memory(p.rss_bytes)}</td>
                <td className="px-4">
                  <button
                    type="button"
                    onClick={() => setSelected(p)}
                    className="border border-border bg-background px-2 py-1.5 text-[10px] text-muted-foreground"
                  >
                    REQUER INSTRUMENTAÇÃO
                  </button>
                </td>
                <td className="px-4">
                  <Button
                    variant="outline"
                    size="sm"
                    aria-label={`DETALHES ${p.name} ${p.pid}`}
                    onClick={() => setSelected(p)}
                  >
                    <Settings2 className="size-3.5" />
                    DETALHES
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!filtered.length && (
          <p className="p-10 text-center text-sm text-muted-foreground">
            NENHUM PROCESSO NESTA SELEÇÃO. AJUSTE OS FILTROS OU AGUARDE A COLETA
            DO AGENTE.
          </p>
        )}
      </div>
      <p className="text-xs leading-5 text-muted-foreground">
        A OCULTAÇÃO NÃO ENCERRA PROCESSOS NEM REMOVE DADOS. A CLASSIFICAÇÃO USA
        NOMES CONHECIDOS DO SISTEMA; APLICAÇÕES PERSONALIZADAS CONTINUAM
        VISÍVEIS.
      </p>
      <Sheet
        open={selected !== null}
        onOpenChange={(open) => {
          if (!open) setSelected(null);
        }}
      >
        <SheetContent className="overflow-y-auto sm:max-w-xl">
          <SheetHeader>
            <SheetTitle>{selected?.name}</SheetTitle>
            <SheetDescription>DETALHES DO PROCESSO OBSERVADO</SheetDescription>
          </SheetHeader>
          {selected && (
            <div className="space-y-5 p-5">
              <dl className="grid grid-cols-2 gap-4 text-xs">
                {[
                  ["HOST", selected.host_name],
                  ["PID", String(selected.pid)],
                  ["SERVIÇO", selected.service || "—"],
                  ["RUNTIME", selected.runtime || "NÃO IDENTIFICADO"],
                  ["MEMÓRIA RSS", memory(selected.rss_bytes)],
                  [
                    "CLASSIFICAÇÃO",
                    selected.system ? "SISTEMA" : "APLICAÇÃO / OUTROS",
                  ],
                ].map(([label, value]) => (
                  <div key={label}>
                    <dt className="text-muted-foreground">{label}</dt>
                    <dd className="mt-1 break-all font-medium">{value}</dd>
                  </div>
                ))}
              </dl>
              <section className="space-y-3 border bg-background p-4">
                <div className="flex items-center justify-between gap-3">
                  <h2 className="text-sm font-semibold">DEEP MONITORING</h2>
                  <input
                    type="checkbox"
                    role="switch"
                    aria-label="DEEP MONITORING"
                    checked={false}
                    disabled
                    aria-describedby="deep-monitoring-requirement"
                    readOnly
                  />
                </div>
                <p
                  id="deep-monitoring-requirement"
                  className="text-xs leading-6 text-muted-foreground"
                >
                  O AGENTE ATUAL NÃO INSTALA SENSORES EM PROCESSOS. ATIVAR OU
                  DESATIVAR INSTRUMENTAÇÃO JAVA/.NET POR ESTA TELA AINDA NÃO É
                  SUPORTADO. PARA RECEBER TRACES, CONFIGURE OPENTELEMETRY NA
                  APLICAÇÃO OU NO COLLECTOR.
                </p>
                <Button variant="outline" asChild>
                  <Link to="/$" params={{ _splat: "ajuda/otlp" }}>
                    <CircleHelp className="size-4" />
                    CONFIGURAR INSTRUMENTAÇÃO
                  </Link>
                </Button>
              </section>
              <div className="flex flex-wrap gap-2">
                <Button
                  variant="outline"
                  onClick={() =>
                    downloadJSON("observa-processo.json", {
                      process: selected,
                      connections,
                    })
                  }
                >
                  <Download className="size-4" />
                  EXPORTAR PROCESSO
                </Button>
                <Button variant="outline" asChild>
                  <Link to="/$" params={{ _splat: "process-mapping" }}>
                    <Server className="size-4" />
                    MAPA DE PROCESSOS
                  </Link>
                </Button>
              </div>
              <h2 className="text-sm font-semibold">
                CONEXÕES OBSERVADAS ({connections.length})
              </h2>
              {connections.slice(0, 100).map((c, i) => (
                <div key={i} className="space-y-1 border p-3 font-mono text-xs">
                  <p className="break-all">
                    {c.local_ip}:{c.local_port} → {c.remote_ip}:{c.remote_port}
                  </p>
                  <p className="text-muted-foreground">{c.state}</p>
                </div>
              ))}
              {!connections.length && (
                <p className="text-xs text-muted-foreground">
                  NENHUMA CONEXÃO RECEBIDA PARA ESTE PID.
                </p>
              )}
              {connections.length > 100 && (
                <p className="text-xs text-muted-foreground">
                  EXIBINDO 100 CONEXÕES. A EXPORTAÇÃO CONTÉM TODAS.
                </p>
              )}
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
