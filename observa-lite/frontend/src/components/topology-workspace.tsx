import { collapseDeviceInterfaces } from "@/lib/topology";
import { isSystemProcess } from "@/lib/processes";
import { Input } from "@/components/ui/input";
import { useState, type ReactNode } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  Boxes,
  Server,
  Workflow,
  AlertTriangle,
  Cloud,
  Bot,
  Network,
  ChevronRight,
  RefreshCw,
  ArrowLeft,
  Layers,
  Download,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { request, type TopologyMap, type Snapshot } from "@/lib/api";
import { downloadJSON } from "@/lib/workspace";
const views = [
  {
    id: "network",
    title: "Mapa de rede",
    description: "Equipamentos, hosts, aplicações e vizinhos LLDP observados",
    icon: Network,
  },
  {
    id: "all",
    title: "Visão unificada",
    description: "Entidades e relações observadas",
    icon: Boxes,
  },
  {
    id: "infrastructure",
    title: "Infraestrutura",
    description: "Hosts, processos e conexões",
    icon: Server,
  },
  {
    id: "problems",
    title: "Problemas",
    description: "Evidências de regras e serviços",
    icon: AlertTriangle,
  },
  {
    id: "services",
    title: "Dependências de serviços",
    description: "Chamadas observadas nos traces",
    icon: Workflow,
  },
  {
    id: "kubernetes",
    title: "Kubernetes",
    description: "Clusters, namespaces e pods recebidos por OTLP",
    icon: Boxes,
  },
  {
    id: "aws",
    title: "AWS",
    description: "Instâncias EC2 do inventário",
    icon: Cloud,
  },
  {
    id: "azure",
    title: "Azure",
    description: "Recursos do azure resource manager",
    icon: Cloud,
  },
  {
    id: "gcp",
    title: "Google cloud",
    description: "Instâncias compute engine",
    icon: Cloud,
  },
  {
    id: "ai",
    title: "AI Observability",
    description: "Serviços com atributos gen_ai recebidos",
    icon: Bot,
  },
  {
    id: "devices",
    title: "Network devices",
    description: "Equipamentos e interfaces IF-MIB",
    icon: Network,
  },
];
export function groupTopology(
  map: TopologyMap,
  groupBy: "none" | "kind" | "host",
  expanded: string[],
) {
  if (groupBy === "none")
    return {
      graph: map,
      groups: [] as { id: string; name: string; count: number }[],
    };
  const groups = new Map<string, typeof map.nodes>();
  const membership = new Map<string, string>();
  for (const n of map.nodes) {
    const name =
      groupBy === "kind" ? n.kind : n.host || "Sem host / Localização";
    const id = `group:${groupBy}:${name}`;
    const members = groups.get(id) ?? [];
    members.push(n);
    groups.set(id, members);
    membership.set(n.id, id);
  }
  const nodes: TopologyMap["nodes"] = [];
  const members = (id: string) => groups.get(id)!;
  for (const [id, items] of groups) {
    if (expanded.includes(id)) nodes.push(...items);
    else
      nodes.push({
        id,
        name: `${groupBy === "kind" ? items[0]!.kind : items[0]!.host || "Sem host / Localização"} (${items.length})`,
        kind: "group",
        status: "unknown",
        instances: items.length,
      });
  }
  const edgeMap = new Map<string, TopologyMap["edges"][number]>();
  for (const e of map.edges) {
    const fg = membership.get(e.from);
    const tg = membership.get(e.to);
    if (!fg || !tg) continue;
    const from = expanded.includes(fg) ? e.from : fg;
    const to = expanded.includes(tg) ? e.to : tg;
    if (from === to) continue;
    const key = `${from}→${to}:${e.kind}`;
    const existing = edgeMap.get(key);
    if (existing) existing.count += e.count;
    else edgeMap.set(key, { ...e, id: key, from, to });
  }
  return {
    graph: { nodes, edges: [...edgeMap.values()] },
    groups: [...groups.keys()].map((id) => ({
      id,
      name:
        groupBy === "kind"
          ? members(id)[0]!.kind
          : members(id)[0]!.host || "Sem host / Localização",
      count: members(id).length,
    })),
  };
}
export function TopologyWorkspace({
  path,
  demo,
  snapshot,
  renderMap,
}: {
  path: string;
  demo: boolean;
  snapshot: Snapshot;
  renderMap: (map: TopologyMap, onExpand: (id: string) => void) => ReactNode;
}) {
  const navigate = useNavigate();
  const id = path.split("/")[1] || "";
  const selected = views.find((v) => v.id === id);
  const [groupBy, setGroupBy] = useState<"none" | "kind" | "host">("none");
  const [expanded, setExpanded] = useState<string[]>([]);
  const [entitySearch, setEntitySearch] = useState("");
  const [showSystem, setShowSystem] = useState(false);
  const [minutes, setMinutes] = useState("60");
  const query = useQuery({
    queryKey: ["topology", id, minutes],
    queryFn: () => {
      const to = new Date();
      const from = new Date(to.getTime() - Number(minutes) * 60000);
      return request<TopologyMap & { truncated: boolean }>(
        `/api/v1/topology?${new URLSearchParams({ view: id, from: from.toISOString(), to: to.toISOString() })}`,
      );
    },
    enabled: !!selected && !demo,
    retry: false,
    refetchInterval: 30000,
  });
  const map =
    query.data ??
    (demo && ["all", "services"].includes(id)
      ? snapshot.service_map
      : { nodes: [], edges: [] });
  const systemIds = new Set<string>(),
    applicationIds = new Set<string>();
  for (const host of snapshot.hosts)
    for (const process of host.processes) {
      const key = `process/svc:${host.id}:${process.service}`;
      (isSystemProcess(process) ? systemIds : applicationIds).add(key);
    }
  const visibleNodes = map.nodes.filter(
    (node) =>
      (showSystem || !systemIds.has(node.id) || applicationIds.has(node.id)) &&
      `${node.name} ${node.id} ${node.kind} ${node.host || ""}`
        .toLowerCase()
        .includes(entitySearch.toLowerCase()),
  );
  const ids = new Set(visibleNodes.map((node) => node.id));
  const visibleIDs = new Set(ids);
  for (const [parent, items] of Object.entries(map.interface_inventory ?? {})) {
    if (ids.has(parent)) for (const item of items) visibleIDs.add(item.id);
  }
  const visibleMap = {
    interface_inventory: Object.fromEntries(
      Object.entries(map.interface_inventory ?? {}).filter(([parent]) =>
        ids.has(parent),
      ),
    ),
    nodes: visibleNodes,
    edges: map.edges.filter(
      (edge) => visibleIDs.has(edge.from) && visibleIDs.has(edge.to),
    ),
  };
  const grouped = groupTopology(
    groupBy === "none"
      ? visibleMap
      : collapseDeviceInterfaces(visibleMap).graph,
    groupBy,
    expanded,
  );
  return (
    <div
      className={
        selected ? "grid gap-5 xl:grid-cols-[190px_minmax(0,1fr)]" : ""
      }
    >
      {selected && (
        <aside className="topology-view-nav sticky top-32 hidden h-fit border bg-surface xl:block">
          <div className="border-b bg-surface-raised px-3 py-3 text-xs font-semibold">
            Visões de topologia
          </div>
          <nav aria-label="Visões de topologia" className="space-y-1 p-2">
            {views.map((view) => (
              <Link
                key={view.id}
                to="/$"
                params={{ _splat: `topologia/${view.id}` }}
                aria-current={view.id === id ? "page" : undefined}
                className={`flex items-center gap-2 border px-2 py-2 text-[10px] ${view.id === id ? "border-primary bg-accent text-foreground" : "border-transparent bg-background text-muted-foreground hover:border-border hover:text-foreground"}`}
              >
                <view.icon className="size-4 shrink-0 text-primary" />
                <span>{view.title}</span>
              </Link>
            ))}
          </nav>
        </aside>
      )}
      <div className="min-w-0 space-y-5">
        <header className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <p className="mb-2 text-xs text-primary">Topologia</p>
            <h1 className="text-2xl font-semibold">
              {selected?.title || "Visões do ambiente"}
            </h1>
            <p className="mt-2 text-sm text-muted-foreground">
              {selected?.description ||
                "Explore a infraestrutura, as aplicações, as clouds e os equipamentos de rede."}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {selected && (
              <>
                <Button variant="outline" asChild>
                  <Link to="/$" params={{ _splat: "topologia" }}>
                    <ArrowLeft className="size-4" />
                    Todas as visões
                  </Link>
                </Button>
                <Button
                  variant="outline"
                  disabled={demo || query.isFetching}
                  onClick={() => query.refetch()}
                >
                  <RefreshCw className="size-4" />
                  Atualizar
                </Button>
              </>
            )}
            <Button variant="outline" asChild>
              <Link to="/$" params={{ _splat: "clouds" }}>
                <Cloud className="size-4" />
                Conectar cloud
              </Link>
            </Button>
            <Button variant="outline" asChild>
              <Link to="/$" params={{ _splat: "dispositivos-rede" }}>
                <Network className="size-4" />
                Equipamentos
              </Link>
            </Button>
          </div>
        </header>
        {!selected ? (
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {views.map((view) => (
              <Link
                key={view.id}
                to="/$"
                params={{ _splat: `topologia/${view.id}` }}
                className="topology-view-card group flex min-h-44 flex-col border bg-surface p-5 hover:border-primary"
              >
                <div className="mb-5 flex items-center justify-between">
                  <span className="topology-icon-tile">
                    <view.icon className="size-6" />
                  </span>
                  <ChevronRight className="size-4 text-muted-foreground" />
                </div>
                <h2 className="text-sm font-semibold">{view.title}</h2>
                <p className="mt-2 text-xs leading-5 text-muted-foreground">
                  {view.description}
                </p>
              </Link>
            ))}
          </div>
        ) : (
          <>
            <div className="flex flex-wrap items-center gap-3 border bg-surface p-3">
              <Select
                value={id}
                onValueChange={(next) => {
                  void navigate({
                    to: "/$",
                    params: { _splat: `topologia/${next}` },
                  });
                }}
              >
                <SelectTrigger aria-label="Visão do mapa" className="w-64">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {views.map((v) => (
                    <SelectItem key={v.id} value={v.id}>
                      <span className="flex items-center gap-2">
                        <v.icon className="size-4 text-primary" />
                        {v.title}
                      </span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select value={minutes} onValueChange={setMinutes}>
                <SelectTrigger aria-label="Período OTLP" className="w-40">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="15">15 Minutos</SelectItem>
                  <SelectItem value="60">1 Hora</SelectItem>
                  <SelectItem value="360">6 Horas</SelectItem>
                  <SelectItem value="1440">24 Horas</SelectItem>
                </SelectContent>
              </Select>
              <Input
                aria-label="Buscar entidade no mapa"
                placeholder="Buscar nome, ID ou localização"
                className="max-w-xs"
                value={entitySearch}
                onChange={(e) => setEntitySearch(e.target.value)}
              />
              <label className="flex items-center gap-2 text-[10px]">
                <input
                  type="checkbox"
                  checked={showSystem}
                  onChange={(e) => setShowSystem(e.target.checked)}
                />
                Incluir processos de sistema
              </label>
              <label className="text-xs">Agrupar por</label>
              <Select
                value={groupBy}
                onValueChange={(v) => {
                  setGroupBy(v as typeof groupBy);
                  setExpanded([]);
                }}
              >
                <SelectTrigger aria-label="Agrupar mapa" className="w-52">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="kind">Tipo de entidade</SelectItem>
                  <SelectItem value="host">Host / localização</SelectItem>
                  <SelectItem value="none">Sem agrupamento</SelectItem>
                </SelectContent>
              </Select>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setExpanded(grouped.groups.map((g) => g.id))}
              >
                <Layers className="size-3.5" />
                Expandir grupos
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setExpanded([])}
              >
                Recolher grupos
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() =>
                  downloadJSON("observa-topologia.json", visibleMap)
                }
              >
                <Download className="size-3.5" />
                Exportar
              </Button>
            </div>
            <div className="flex flex-wrap gap-2">
              {grouped.groups.map((group) => (
                <button
                  key={group.id}
                  className="border bg-surface px-3 py-2 text-[10px] hover:border-primary"
                  aria-pressed={expanded.includes(group.id)}
                  onClick={() =>
                    setExpanded((previous) =>
                      previous.includes(group.id)
                        ? previous.filter((v) => v !== group.id)
                        : [...previous, group.id],
                    )
                  }
                >
                  {group.name} · {group.count} ·{" "}
                  {expanded.includes(group.id) ? "Recolher" : "Expandir"}
                </button>
              ))}
            </div>
            {query.isLoading ? (
              <p role="status">Carregando topologia…</p>
            ) : query.error ? (
              <div role="alert" className="border p-5 text-sm text-danger">
                {query.error.message}
                <Button variant="outline" onClick={() => query.refetch()}>
                  Tentar novamente
                </Button>
              </div>
            ) : visibleMap.nodes.length ? (
              renderMap(grouped.graph, (id) =>
                setExpanded((previous) =>
                  previous.includes(id) ? previous : [...previous, id],
                ),
              )
            ) : (
              <div className="border border-dashed bg-surface p-12 text-center">
                <Network className="mx-auto mb-4 size-10 text-primary" />
                <h2 className="font-semibold">Nenhuma entidade nesta visão</h2>
                <p className="mx-auto mt-3 max-w-xl text-xs leading-6 text-muted-foreground">
                  Configure a integração e verifique a coleta. As visões
                  kubernetes e AI usam atributos explícitos dos spans OTLP; não
                  descobrem clusters ou modelos automaticamente.
                </p>
              </div>
            )}
            {query.data?.truncated && (
              <p className="text-xs text-warning">
                A API limitou o mapa a 500 nós / 2000 relações. A exibição do
                canvas tem limite adicional de 200 nós.
              </p>
            )}
            <p className="text-xs leading-5 text-muted-foreground">
              O período afeta os dados OTLP. clouds e equipamentos usam o último
              inventário coletado. relações de inventário são contenção;
              chamadas vêm dos traces. problemas de regras não significam causa
              raiz. recursos retidos após falha de coleta aparecem como stale.
            </p>
          </>
        )}
      </div>
    </div>
  );
}
