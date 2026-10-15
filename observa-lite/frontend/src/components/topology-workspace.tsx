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
    id: "all",
    title: "VISÃO UNIFICADA",
    description: "ENTIDADES E RELAÇÕES OBSERVADAS",
    icon: Boxes,
  },
  {
    id: "infrastructure",
    title: "INFRAESTRUTURA",
    description: "HOSTS, PROCESSOS E CONEXÕES",
    icon: Server,
  },
  {
    id: "problems",
    title: "PROBLEMAS",
    description: "EVIDÊNCIAS DE REGRAS E SERVIÇOS",
    icon: AlertTriangle,
  },
  {
    id: "services",
    title: "DEPENDÊNCIAS DE SERVIÇOS",
    description: "CHAMADAS OBSERVADAS NOS TRACES",
    icon: Workflow,
  },
  {
    id: "kubernetes",
    title: "KUBERNETES",
    description: "CLUSTERS, NAMESPACES E PODS RECEBIDOS POR OTLP",
    icon: Boxes,
  },
  {
    id: "aws",
    title: "AWS",
    description: "INSTÂNCIAS EC2 DO INVENTÁRIO",
    icon: Cloud,
  },
  {
    id: "azure",
    title: "AZURE",
    description: "RECURSOS DO AZURE RESOURCE MANAGER",
    icon: Cloud,
  },
  {
    id: "gcp",
    title: "GOOGLE CLOUD",
    description: "INSTÂNCIAS COMPUTE ENGINE",
    icon: Cloud,
  },
  {
    id: "ai",
    title: "AI OBSERVABILITY",
    description: "SERVIÇOS COM ATRIBUTOS GEN_AI RECEBIDOS",
    icon: Bot,
  },
  {
    id: "devices",
    title: "NETWORK DEVICES",
    description: "EQUIPAMENTOS E INTERFACES IF-MIB",
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
      groupBy === "kind" ? n.kind : n.host || "SEM HOST / LOCALIZAÇÃO";
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
        name: `${groupBy === "kind" ? items[0]!.kind : items[0]!.host || "SEM HOST / LOCALIZAÇÃO"} (${items.length})`,
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
          : members(id)[0]!.host || "SEM HOST / LOCALIZAÇÃO",
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
  const [groupBy, setGroupBy] = useState<"none" | "kind" | "host">("kind");
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
  const visibleMap = {
    nodes: visibleNodes,
    edges: map.edges.filter((edge) => ids.has(edge.from) && ids.has(edge.to)),
  };
  const grouped = groupTopology(visibleMap, groupBy, expanded);
  return (
    <div
      className={
        selected ? "grid gap-5 xl:grid-cols-[190px_minmax(0,1fr)]" : ""
      }
    >
      {selected && (
        <aside className="topology-view-nav sticky top-32 hidden h-fit border bg-surface xl:block">
          <div className="border-b bg-surface-raised px-3 py-3 text-xs font-semibold">
            VISÕES DE TOPOLOGIA
          </div>
          <nav aria-label="VISÕES DE TOPOLOGIA" className="space-y-1 p-2">
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
            <p className="mb-2 text-xs text-primary">TOPOLOGIA</p>
            <h1 className="text-2xl font-semibold">
              {selected?.title || "VISÕES DO AMBIENTE"}
            </h1>
            <p className="mt-2 text-sm text-muted-foreground">
              {selected?.description ||
                "EXPLORE A INFRAESTRUTURA, AS APLICAÇÕES, AS CLOUDS E OS EQUIPAMENTOS DE REDE."}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {selected && (
              <>
                <Button variant="outline" asChild>
                  <Link to="/$" params={{ _splat: "topologia" }}>
                    <ArrowLeft className="size-4" />
                    TODAS AS VISÕES
                  </Link>
                </Button>
                <Button
                  variant="outline"
                  disabled={demo || query.isFetching}
                  onClick={() => query.refetch()}
                >
                  <RefreshCw className="size-4" />
                  ATUALIZAR
                </Button>
              </>
            )}
            <Button variant="outline" asChild>
              <Link to="/$" params={{ _splat: "clouds" }}>
                <Cloud className="size-4" />
                CONECTAR CLOUD
              </Link>
            </Button>
            <Button variant="outline" asChild>
              <Link to="/$" params={{ _splat: "dispositivos-rede" }}>
                <Network className="size-4" />
                EQUIPAMENTOS
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
                <SelectTrigger aria-label="VISÃO DO MAPA" className="w-64">
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
                <SelectTrigger aria-label="PERÍODO OTLP" className="w-40">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="15">15 MINUTOS</SelectItem>
                  <SelectItem value="60">1 HORA</SelectItem>
                  <SelectItem value="360">6 HORAS</SelectItem>
                  <SelectItem value="1440">24 HORAS</SelectItem>
                </SelectContent>
              </Select>
              <Input
                aria-label="BUSCAR ENTIDADE NO MAPA"
                placeholder="BUSCAR NOME, ID OU LOCALIZAÇÃO"
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
                INCLUIR PROCESSOS DE SISTEMA
              </label>
              <label className="text-xs">AGRUPAR POR</label>
              <Select
                value={groupBy}
                onValueChange={(v) => {
                  setGroupBy(v as typeof groupBy);
                  setExpanded([]);
                }}
              >
                <SelectTrigger aria-label="AGRUPAR MAPA" className="w-52">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="kind">TIPO DE ENTIDADE</SelectItem>
                  <SelectItem value="host">HOST / LOCALIZAÇÃO</SelectItem>
                  <SelectItem value="none">SEM AGRUPAMENTO</SelectItem>
                </SelectContent>
              </Select>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setExpanded(grouped.groups.map((g) => g.id))}
              >
                <Layers className="size-3.5" />
                EXPANDIR GRUPOS
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setExpanded([])}
              >
                RECOLHER GRUPOS
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() =>
                  downloadJSON("observa-topologia.json", visibleMap)
                }
              >
                <Download className="size-3.5" />
                EXPORTAR
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
                  {expanded.includes(group.id) ? "RECOLHER" : "EXPANDIR"}
                </button>
              ))}
            </div>
            {query.isLoading ? (
              <p role="status">CARREGANDO TOPOLOGIA…</p>
            ) : query.error ? (
              <div role="alert" className="border p-5 text-sm text-danger">
                {query.error.message}
                <Button variant="outline" onClick={() => query.refetch()}>
                  TENTAR NOVAMENTE
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
                <h2 className="font-semibold">NENHUMA ENTIDADE NESTA VISÃO</h2>
                <p className="mx-auto mt-3 max-w-xl text-xs leading-6 text-muted-foreground">
                  CONFIGURE A INTEGRAÇÃO E VERIFIQUE A COLETA. AS VISÕES
                  KUBERNETES E AI USAM ATRIBUTOS EXPLÍCITOS DOS SPANS OTLP; NÃO
                  DESCOBREM CLUSTERS OU MODELOS AUTOMATICAMENTE.
                </p>
              </div>
            )}
            {query.data?.truncated && (
              <p className="text-xs text-warning">
                A API LIMITOU O MAPA A 500 NÓS / 2000 RELAÇÕES. A EXIBIÇÃO DO
                CANVAS TEM LIMITE ADICIONAL DE 200 NÓS.
              </p>
            )}
            <p className="text-xs leading-5 text-muted-foreground">
              O PERÍODO AFETA OS DADOS OTLP. CLOUDS E EQUIPAMENTOS USAM O ÚLTIMO
              INVENTÁRIO COLETADO. RELAÇÕES DE INVENTÁRIO SÃO CONTENÇÃO;
              CHAMADAS VÊM DOS TRACES. PROBLEMAS DE REGRAS NÃO SIGNIFICAM CAUSA
              RAIZ. RECURSOS RETIDOS APÓS FALHA DE COLETA APARECEM COMO STALE.
            </p>
          </>
        )}
      </div>
    </div>
  );
}
