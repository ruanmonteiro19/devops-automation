import { InventoryEditor } from "./inventory-editor";
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
export function ProcessExplorer({
  snapshot,
  onSaved,
}: {
  snapshot: Snapshot;
  onSaved?: () => void;
}) {
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
        `${p.display_name || p.name} ${p.service} ${p.runtime} ${p.host_name} ${p.pid}`
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
            Infraestrutura
          </p>
          <h1 className="text-2xl font-semibold">Processos de aplicação</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Processos observados pelos agentes. componentes de sistema ocultos
            por padrão.
          </p>
        </div>
        <Button
          variant="outline"
          onClick={() => downloadJSON("observa-processos.json", filtered)}
        >
          <Download className="size-4" />
          Exportar resultados
        </Button>
      </header>
      <div className="grid gap-3 sm:grid-cols-3">
        {[
          ["Aplicações e outros processos", all.length - systemCount],
          ["Hosts", snapshot.hosts.length],
          ["Processos de sistema ocultos", showSystem ? 0 : systemCount],
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
            aria-label="Buscar processos"
            placeholder="Buscar processo, serviço ou PID"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
        <select
          aria-label="Filtrar host"
          className="h-10 max-w-60 border bg-background px-3 text-xs"
          value={host}
          onChange={(e) => setHost(e.target.value)}
        >
          <option value="">Todos os hosts</option>
          {snapshot.hosts.map((h) => (
            <option key={h.id} value={h.id}>
              {h.name}
            </option>
          ))}
        </select>
        <select
          aria-label="Filtrar runtime"
          className="h-10 border bg-background px-3 text-xs"
          value={runtime}
          onChange={(e) => setRuntime(e.target.value)}
        >
          <option value="">Todos os runtimes</option>
          {[...new Set(all.map((p) => p.runtime).filter(Boolean))]
            .sort()
            .map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
        </select>
        <select
          aria-label="Ordenar processos"
          className="h-10 border bg-background px-3 text-xs"
          value={sort}
          onChange={(e) => setSort(e.target.value)}
        >
          <option value="memory">Maior memória RSS</option>
          <option value="name">Nome</option>
          <option value="pid">PID</option>
        </select>
        <label className="flex items-center gap-2 border bg-background px-3 py-2.5 text-xs">
          <input
            type="checkbox"
            checked={showSystem}
            onChange={(e) => setShowSystem(e.target.checked)}
          />
          Mostrar processos de sistema
        </label>
      </section>
      <div className="flex flex-wrap items-center justify-between gap-3 text-xs text-muted-foreground">
        <p>
          {filtered.length} Resultados · {Math.min(limit, filtered.length)}{" "}
          Exibidos
        </p>
        <div className="flex items-center gap-2">
          <span>Exibir</span>
          <select
            aria-label="Limite de processos"
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
            Limpar filtros
          </Button>
        </div>
      </div>
      <div className="overflow-x-auto border bg-surface">
        <table className="w-full min-w-[850px] text-left text-xs">
          <thead className="bg-surface-raised">
            <tr>
              {[
                "Processo",
                "PID",
                "Serviço",
                "Runtime",
                "Host",
                "Memória RSS",
                "Deep monitoring",
                "Ações",
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
                    {p.display_name || p.name}
                  </button>
                  {p.system && (
                    <p className="mt-1 text-[10px] text-muted-foreground">
                      Sistema
                    </p>
                  )}
                </td>
                <td className="px-4">{p.pid}</td>
                <td className="px-4">{p.service || "—"}</td>
                <td className="px-4">{p.runtime || "Não identificado"}</td>
                <td className="px-4">{p.host_name}</td>
                <td className="px-4 tabular-nums">{memory(p.rss_bytes)}</td>
                <td className="px-4">
                  <button
                    type="button"
                    onClick={() => setSelected(p)}
                    className="border border-border bg-background px-2 py-1.5 text-[10px] text-muted-foreground"
                  >
                    Requer instrumentação
                  </button>
                </td>
                <td className="px-4">
                  <Button
                    variant="outline"
                    size="sm"
                    aria-label={`DETALHES ${p.display_name || p.name} ${p.pid}`}
                    onClick={() => setSelected(p)}
                  >
                    <Settings2 className="size-3.5" />
                    Detalhes
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!filtered.length && (
          <p className="p-10 text-center text-sm text-muted-foreground">
            Nenhum processo nesta seleção. ajuste os filtros ou aguarde a coleta
            do agente.
          </p>
        )}
      </div>
      <p className="text-xs leading-5 text-muted-foreground">
        A ocultação não encerra processos nem remove dados. A classificação usa
        nomes conhecidos do sistema; aplicações personalizadas continuam
        visíveis.
      </p>
      <Sheet
        open={selected !== null}
        onOpenChange={(open) => {
          if (!open) setSelected(null);
        }}
      >
        <SheetContent className="overflow-y-auto sm:max-w-xl">
          <SheetHeader>
            <SheetTitle>{selected?.display_name || selected?.name}</SheetTitle>
            <SheetDescription>Detalhes do processo observado</SheetDescription>
          </SheetHeader>
          {selected && (
            <div className="space-y-5 p-5">
              <InventoryEditor
                host={snapshot.hosts.find((h) => h.id === selected.host_id)!}
                process={selected}
                groups={[]}
                onSaved={() => {
                  setSelected(null);
                  onSaved?.();
                }}
              />
              <dl className="grid grid-cols-2 gap-4 text-xs">
                {[
                  ["Host", selected.host_name],
                  ["PID", String(selected.pid)],
                  ["Serviço", selected.service || "—"],
                  ["Runtime", selected.runtime || "Não identificado"],
                  ["Memória RSS", memory(selected.rss_bytes)],
                  [
                    "Classificação",
                    selected.system ? "Sistema" : "Aplicação / Outros",
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
                  <h2 className="text-sm font-semibold">Deep monitoring</h2>
                  <input
                    type="checkbox"
                    role="switch"
                    aria-label="Deep monitoring"
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
                  O agente atual não instala sensores em processos. Ativar ou
                  desativar instrumentação Java/.NET por esta tela ainda não é
                  suportado. Para receber traces, configure opentelemetry na
                  aplicação ou no collector.
                </p>
                <Button variant="outline" asChild>
                  <Link to="/$" params={{ _splat: "ajuda/otlp" }}>
                    <CircleHelp className="size-4" />
                    Configurar instrumentação
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
                  Exportar processo
                </Button>
                <Button variant="outline" asChild>
                  <Link to="/$" params={{ _splat: "process-mapping" }}>
                    <Server className="size-4" />
                    Mapa de processos
                  </Link>
                </Button>
              </div>
              <h2 className="text-sm font-semibold">
                Conexões observadas ({connections.length})
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
                  Nenhuma conexão recebida para este PID.
                </p>
              )}
              {connections.length > 100 && (
                <p className="text-xs text-muted-foreground">
                  Exibindo 100 conexões. A exportação contém todas.
                </p>
              )}
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
