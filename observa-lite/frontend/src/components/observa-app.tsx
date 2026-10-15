import { ProcessExplorer } from "@/components/process-explorer";
import { applicationProcesses } from "@/lib/processes";
import { MonitorManager } from "@/components/monitor-manager";
import { downloadJSON, menuIconColor } from "@/lib/workspace";
import {
  WorkspaceAccess,
  WorkspaceTools,
  WorkspaceWelcome,
  WorkspacePreferences,
  HelpCenter,
} from "@/components/workspace-experience";
import {
  filterTopology,
  layoutTopology,
  type TopologyLayout,
} from "@/lib/topology";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useLocation, useNavigate } from "@tanstack/react-router";
import {
  Activity,
  AlertTriangle,
  Bell,
  Bot,
  Boxes,
  ChevronDown,
  ChevronRight,
  CircleHelp,
  Clock3,
  Cloud,
  Code2,
  Database,
  Gauge,
  GitBranch,
  Globe2,
  HeartPulse,
  KeyRound,
  LayoutDashboard,
  LifeBuoy,
  ListFilter,
  LockKeyhole,
  Menu,
  Network,
  PanelLeftClose,
  PanelRight,
  Play,
  Plus,
  Radio,
  RefreshCw,
  Search,
  Server,
  Settings,
  ShieldCheck,
  Star,
  Trash2,
  Users,
  Workflow,
  X,
  Pencil,
  type LucideIcon,
} from "lucide-react";
import {
  Background,
  Controls,
  Handle,
  MiniMap,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import {
  memo,
  useEffect,
  useMemo,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  TooltipProvider,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  api,
  ApiError,
  readToken,
  saveToken,
  type Dashboard,
  type DataSource,
  type GatewayEndpoint,
  type MapNode,
  type MonitorTemplate,
  type Snapshot,
  type TopologyMap,
} from "@/lib/api";
import { createDemoSnapshot } from "@/lib/demo-data";
import { TelemetryPage } from "@/components/telemetry-page";
import { APMProblemsPage } from "@/components/apm-problems-page";
import { OperationalDashboards } from "@/components/operational-dashboards";
import { SLOPage } from "@/components/slo-page";
import { APMPage } from "@/components/apm-page";
import { LogsPage } from "@/components/logs-page";
import { navText, validLanguage, type ObservaLanguage } from "@/lib/i18n";
import {
  extensionManifest,
  type ExtensionCategory,
  type ExtensionManifest,
} from "@/lib/extensions";
import { cn } from "@/lib/utils";

type NavItem = {
  label: string;
  path: string;
  icon: LucideIcon;
  favorite?: boolean;
};
type NavGroup = { label: string; items: NavItem[] };

const navGroups: NavGroup[] = [
  {
    label: "Workspace",
    items: (
      [
        ["Visão geral", "", LayoutDashboard],
        ["Dashboards", "dashboards", PanelRight],
        ["Problemas APM", "problemas-apm", AlertTriangle],
      ] satisfies [string, string, LucideIcon][]
    ).map(toNav),
  },
  {
    label: "Aplicações & serviços",
    items: (
      [
        ["Serviços", "servicos", Boxes],
        ["Mapa de serviços", "service-mapping", Workflow],
        ["Traces distribuídos", "traces", Activity],
        ["Erros & exceções", "erros", AlertTriangle],
        ["Requisições", "requisicoes", Radio],
      ] satisfies [string, string, LucideIcon][]
    ).map(toNav),
  },
  {
    label: "Infraestrutura",
    items: (
      [
        ["Hosts", "hosts", Server],
        ["Processos", "processos", Activity],
        ["Mapa de processos", "process-mapping", GitBranch],
        ["Mapa de rede", "network-mapping", Network],
      ] satisfies [string, string, LucideIcon][]
    ).map(toNav),
  },
  {
    label: "Explorar dados",
    items: (
      [
        ["Logs OTLP", "telemetria-logs", ListFilter],
        ["Métricas", "metricas", Gauge],
        ["Logs Loki", "logs", Code2],
        ["Fontes de dados", "data-sources", Database],
      ] satisfies [string, string, LucideIcon][]
    ).map(toNav),
  },
  {
    label: "Confiabilidade",
    items: (
      [
        ["Objetivos SLO", "slos", HeartPulse],
        ["Regras de alerta", "correlacao", Bell],
        ["Monitores HTTP / TCP", "templates", Globe2],
        ["Eventos de monitores", "incidentes", Clock3],
      ] satisfies [string, string, LucideIcon][]
    ).map(toNav),
  },
  {
    label: "Gerenciar",
    items: (
      [
        ["Agentes", "agentes", Radio],
        ["Gateways", "gateways", Network],
        ["Host Groups", "host-groups", Server],
        ["Network Zones", "network-zones", Network],
        ["Extensões", "extensoes", Boxes],
        ["Acesso & token", "chaves-api", KeyRound],
        ["Preferências", "configuracoes", Settings],
        ["Central de ajuda", "ajuda", CircleHelp],
      ] satisfies [string, string, LucideIcon][]
    ).map(toNav),
  },
];

function toNav(tuple: [string, string, LucideIcon, boolean?]): NavItem {
  return {
    label: tuple[0],
    path: tuple[1],
    icon: tuple[2],
    favorite: tuple[3] ?? false,
  };
}
const labels = new Map(
  navGroups.flatMap((group) =>
    group.items.map((item) => [item.path, item.label]),
  ),
);

function routePath(path: string) {
  return path ? `/${path}` : "/";
}
function useCurrentPath() {
  return useLocation({
    select: (location) =>
      location.pathname.replace(/^\//, "").replace(/\/$/, ""),
  });
}

export function ObservaApp() {
  const queryClient = useQueryClient();
  const path = useCurrentPath();
  const [mobileNav, setMobileNav] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const [navSearch, setNavSearch] = useState("");
  const [globalSearch, setGlobalSearch] = useState("");
  const [demo, setDemo] = useState(false);
  const [hydrated, setHydrated] = useState(false);
  const [polling, setPolling] = useState(30_000);
  const [language, setLanguage] = useState<ObservaLanguage>(() =>
    validLanguage(
      typeof window === "undefined"
        ? null
        : window.localStorage.getItem("observa-ui-language"),
    ),
  );
  const changeLanguage = (next: ObservaLanguage) => {
    setLanguage(next);
    window.localStorage.setItem("observa-ui-language", next);
    document.documentElement.lang = next;
  };

  useEffect(() => {
    setHydrated(true);
  }, []);
  const snapshotQuery = useQuery({
    queryKey: ["snapshot", demo],
    queryFn: () =>
      demo ? Promise.resolve(createDemoSnapshot()) : api.snapshot(),
    enabled: hydrated,
    retry: false,
    refetchInterval: demo ? false : polling,
  });
  const unauthorized =
    snapshotQuery.error instanceof ApiError &&
    snapshotQuery.error.status === 401;
  const unreachable =
    snapshotQuery.error instanceof ApiError &&
    snapshotQuery.error.status !== 401;

  function enterDemo() {
    setDemo(true);
    void queryClient.invalidateQueries({ queryKey: ["snapshot"] });
  }
  function exitDemo() {
    setDemo(false);
    void queryClient.invalidateQueries({ queryKey: ["snapshot"] });
  }

  return (
    <TooltipProvider delayDuration={200}>
      <div className="min-h-screen bg-background text-foreground">
        {demo && <DemoBanner onExit={exitDemo} />}
        <div className={cn("flex min-h-screen", demo && "pt-9")}>
          <aside
            className={cn(
              "fixed inset-y-0 left-0 z-40 hidden border-r border-sidebar-border bg-sidebar transition-[width] duration-200 md:flex md:flex-col",
              demo && "top-9",
              collapsed ? "w-16" : "w-[248px]",
            )}
          >
            <SidebarNav
              collapsed={collapsed}
              search={navSearch}
              setSearch={setNavSearch}
              current={path}
              language={language}
            />
          </aside>
          <div
            className={cn(
              "min-w-0 flex-1 transition-[margin] duration-200",
              collapsed ? "md:ml-16" : "md:ml-[248px]",
            )}
          >
            <Topbar
              current={path}
              collapsed={collapsed}
              onCollapse={() => setCollapsed((value) => !value)}
              onMobile={() => setMobileNav(true)}
              search={globalSearch}
              setSearch={setGlobalSearch}
              polling={polling}
              setPolling={setPolling}
              onRefresh={() => void snapshotQuery.refetch()}
              isRefreshing={snapshotQuery.isFetching}
              language={language}
              onLanguage={changeLanguage}
            />
            <WorkspaceTools
              snapshot={snapshotQuery.data}
              demo={demo}
              current={path.replace(/\/novo$/, "")}
            />
            <main className="workspace-main min-h-[calc(100vh-110px)] p-4 lg:p-6 xl:p-7">
              {!hydrated || snapshotQuery.isLoading ? (
                <LoadingState />
              ) : snapshotQuery.data ? (
                <PageRouter
                  path={path}
                  snapshot={snapshotQuery.data}
                  demo={demo}
                  globalSearch={globalSearch}
                />
              ) : (
                <ConnectionGate
                  unauthorized={unauthorized}
                  unreachable={unreachable}
                  error={snapshotQuery.error}
                  onDemo={enterDemo}
                  onRetry={() => void snapshotQuery.refetch()}
                />
              )}
            </main>
          </div>
        </div>
        <Sheet open={mobileNav} onOpenChange={setMobileNav}>
          <SheetContent
            side="left"
            className="w-[88vw] max-w-[320px] border-sidebar-border bg-sidebar p-0"
          >
            <SheetHeader className="sr-only">
              <SheetTitle>Navegação</SheetTitle>
              <SheetDescription>Menu principal</SheetDescription>
            </SheetHeader>
            <SidebarNav
              collapsed={false}
              search={navSearch}
              setSearch={setNavSearch}
              current={path}
              onNavigate={() => setMobileNav(false)}
              language={language}
            />
          </SheetContent>
        </Sheet>
      </div>
    </TooltipProvider>
  );
}

function DemoBanner({ onExit }: { onExit: () => void }) {
  return (
    <div className="fixed inset-x-0 top-0 z-[60] flex h-9 items-center justify-center gap-3 bg-demo px-3 text-xs font-semibold text-demo-foreground">
      <span className="rounded-sm border border-current px-1.5 py-0.5 text-[10px]">
        DEMO
      </span>
      <span className="truncate">
        Dados ilustrativos — nenhuma telemetria real está sendo exibida.
      </span>
      <Button
        variant="ghost"
        size="sm"
        className="h-7 text-demo-foreground hover:bg-demo-foreground/10"
        onClick={onExit}
      >
        Sair
      </Button>
    </div>
  );
}

function SidebarNav({
  collapsed,
  search,
  setSearch,
  current,
  onNavigate,
  language,
}: {
  collapsed: boolean;
  search: string;
  setSearch: (value: string) => void;
  current: string;
  onNavigate?: (() => void) | undefined;
  language: ObservaLanguage;
}) {
  const [favorites, setFavorites] = useState<string[]>(() => {
    try {
      const raw: unknown = JSON.parse(
        localStorage.getItem("observa-favorites-v1") ||
          '["servicos","dashboards"]',
      );
      return Array.isArray(raw)
        ? raw.filter((x): x is string => typeof x === "string")
        : [];
    } catch {
      return [];
    }
  });
  function toggleFavorite(path: string) {
    setFavorites((previous) => {
      const next = previous.includes(path)
        ? previous.filter((x) => x !== path)
        : [...previous, path];
      try {
        localStorage.setItem("observa-favorites-v1", JSON.stringify(next));
      } catch {
        /* browser storage optional */
      }
      return next;
    });
  }
  const filtered = navGroups
    .map((group) => ({
      ...group,
      items: group.items.filter((item) =>
        item.label.toLowerCase().includes(search.toLowerCase()),
      ),
    }))
    .filter((group) => group.items.length);
  return (
    <>
      <div className="flex h-16 shrink-0 items-center gap-3 border-b border-sidebar-border px-4">
        <div className="grid size-8 shrink-0 place-items-center rounded-md bg-primary/15 text-primary border border-primary/30">
          <Activity className="size-5" />
        </div>
        {!collapsed && (
          <div>
            <div className="text-xl font-bold tracking-tight leading-tight">
              Observa<span className="text-primary">.</span>
            </div>
            <div className="text-[10px] font-medium uppercase text-muted-foreground">
              Observabilidade
            </div>
          </div>
        )}
      </div>
      {!collapsed && (
        <div className="p-3">
          <div className="relative">
            <Search className="absolute left-2.5 top-2.5 size-4 menu-icon-violet" />
            <Input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={navText("Filtrar navegação", language)}
              className="h-9 border-sidebar-border bg-sidebar-accent pl-8 text-xs"
            />
          </div>
        </div>
      )}
      <nav
        aria-label="Navegação principal"
        className="observa-scrollbar flex-1 overflow-y-auto px-2 pb-4"
      >
        {!collapsed && !search && favorites.length > 0 && (
          <div className="mb-4 mt-2">
            <p className="mb-2 flex items-center gap-2 px-2 text-[10px] font-semibold uppercase tracking-wider text-nav-section">
              <Star className="size-3" />
              Favoritos
            </p>
            {navGroups
              .flatMap((g) => g.items)
              .filter((item) => favorites.includes(item.path))
              .map((item) => (
                <NavLink
                  key={item.path}
                  item={item}
                  active={current === item.path}
                  collapsed={false}
                  onNavigate={onNavigate}
                  language={language}
                  favorite={true}
                  onFavorite={() => toggleFavorite(item.path)}
                />
              ))}
          </div>
        )}
        {filtered.map((group) => (
          <details
            key={`${group.label}-${search ? "search" : "browse"}-${collapsed ? "collapsed" : "expanded"}`}
            open
            className="group/nav mt-3"
          >
            {!collapsed && (
              <summary className="flex cursor-pointer list-none items-center justify-between px-2 pb-2 text-[10px] font-semibold uppercase tracking-wider text-nav-section">
                {navText(group.label, language)}
                <ChevronDown className="size-3 transition group-open/nav:rotate-180" />
              </summary>
            )}
            <div className="space-y-0.5">
              {group.items.map((item) => (
                <NavLink
                  key={item.path || "home"}
                  item={item}
                  active={
                    current === item.path ||
                    (item.path !== "" && current.startsWith(item.path + "/"))
                  }
                  collapsed={collapsed}
                  onNavigate={onNavigate}
                  language={language}
                  favorite={favorites.includes(item.path)}
                  onFavorite={() => toggleFavorite(item.path)}
                />
              ))}
            </div>
          </details>
        ))}
      </nav>
      <div className="border-t border-sidebar-border p-2">
        <div
          className={cn(
            "flex items-center gap-2 rounded-md bg-sidebar-accent p-2",
            collapsed && "justify-center",
          )}
        >
          <span className="relative flex size-2">
            <span className="relative inline-flex size-2 rounded-full bg-success" />
          </span>
          {!collapsed && (
            <span className="text-xs text-muted-foreground">
              Workspace · Observa 0.3
            </span>
          )}
        </div>
      </div>
    </>
  );
}

function NavLink({
  item,
  active,
  collapsed,
  onNavigate,
  language,
  favorite = false,
  onFavorite,
}: {
  item: NavItem;
  active: boolean;
  collapsed: boolean;
  onNavigate?: (() => void) | undefined;
  language: ObservaLanguage;
  favorite?: boolean;
  onFavorite?: () => void;
}) {
  const content = (
    <div
      data-active={active}
      className={cn(
        "nav-menu-item group relative flex items-center rounded-lg transition-colors hover:bg-sidebar-accent",
        active &&
          "bg-primary/10 text-primary ring-1 ring-inset ring-primary/15",
      )}
    >
      <Link
        to="/$"
        params={{ _splat: item.path }}
        onClick={onNavigate}
        aria-current={active ? "page" : undefined}
        aria-label={navText(item.label, language)}
        className={cn(
          "flex h-10 min-w-0 flex-1 items-center gap-2.5 rounded-lg px-3 text-[13px] text-sidebar-foreground hover:text-foreground",
          active && "font-medium text-primary",
          collapsed && "justify-center px-0",
        )}
      >
        <item.icon
          className={cn("size-4 shrink-0", menuIconColor(item.path))}
        />
        {!collapsed && (
          <span className="truncate">{navText(item.label, language)}</span>
        )}
      </Link>
      {!collapsed && onFavorite && (
        <button
          aria-label={`${favorite ? "Remover dos" : "Adicionar aos"} favoritos: ${item.label}`}
          aria-pressed={favorite}
          onClick={onFavorite}
          className={cn(
            "mr-1 rounded p-1.5 text-muted-foreground opacity-0 transition hover:text-primary focus-visible:opacity-100 group-hover:opacity-100",
            favorite && "text-primary opacity-100",
          )}
        >
          <Star className={cn("size-3", favorite && "fill-current")} />
        </button>
      )}
    </div>
  );
  return collapsed ? (
    <Tooltip>
      <TooltipTrigger asChild>{content}</TooltipTrigger>
      <TooltipContent side="right">
        {navText(item.label, language)}
      </TooltipContent>
    </Tooltip>
  ) : (
    content
  );
}

function Topbar({
  current,
  collapsed,
  onCollapse,
  onMobile,
  search,
  setSearch,
  polling,
  setPolling,
  onRefresh,
  isRefreshing,
  language,
  onLanguage,
}: {
  current: string;
  collapsed: boolean;
  onCollapse: () => void;
  onMobile: () => void;
  search: string;
  setSearch: (v: string) => void;
  polling: number;
  setPolling: (v: number) => void;
  onRefresh: () => void;
  isRefreshing: boolean;
  language: ObservaLanguage;
  onLanguage: (v: ObservaLanguage) => void;
}) {
  return (
    <header className="sticky top-0 z-30 flex h-14 items-center gap-2 border-b border-border bg-background/95 px-3 backdrop-blur lg:px-5">
      <Button
        variant="ghost"
        size="icon"
        className="md:hidden"
        onClick={onMobile}
        aria-label="Abrir menu"
      >
        <Menu />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        className="hidden md:inline-flex"
        onClick={onCollapse}
        aria-label={collapsed ? "Expandir menu" : "Recolher menu"}
      >
        <PanelLeftClose
          className={cn("transition-transform", collapsed && "rotate-180")}
        />
      </Button>
      <div className="hidden items-center gap-1 text-xs text-muted-foreground lg:flex">
        <span>Observa Lite</span>
        <ChevronRight className="size-3" />
        <span className="font-medium text-foreground">
          {navText(labels.get(current) ?? "Visão geral", language)}
        </span>
      </div>
      <div className="mx-auto hidden w-full max-w-md md:block">
        <div className="relative">
          <Search className="absolute left-3 top-2.5 size-4 text-muted-foreground" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            className="h-9 bg-surface pl-9 text-xs"
            placeholder={navText(
              "Pesquisar host, serviço, PID ou evento",
              language,
            )}
          />
        </div>
      </div>

      <Select
        value={String(polling)}
        onValueChange={(value) => setPolling(Number(value))}
      >
        <SelectTrigger className="hidden h-8 w-[114px] bg-surface text-xs lg:flex">
          <Clock3 className="size-3" />
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="15000">15 s</SelectItem>
          <SelectItem value="30000">30 s</SelectItem>
          <SelectItem value="60000">1 min</SelectItem>
          <SelectItem value="0">Manual</SelectItem>
        </SelectContent>
      </Select>
      <Select
        value={language}
        onValueChange={(v) => onLanguage(validLanguage(v))}
      >
        <SelectTrigger
          className="h-8 w-[100px] bg-surface text-xs"
          aria-label="Interface language"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="pt-BR">PT-BR</SelectItem>
          <SelectItem value="en-US">English</SelectItem>
          <SelectItem value="es-ES">Español</SelectItem>
        </SelectContent>
      </Select>
      <Button
        variant="ghost"
        size="icon"
        className="size-8"
        onClick={onRefresh}
        aria-label="Atualizar"
      >
        <RefreshCw className={cn(isRefreshing && "animate-spin")} />
      </Button>
      <Button variant="ghost" size="icon" className="size-8" asChild>
        <Link
          to="/$"
          params={{ _splat: "problemas-apm" }}
          aria-label="Abrir problemas"
        >
          <Bell className="menu-icon-orange" />
        </Link>
      </Button>
      <Button
        variant="outline"
        size="icon"
        className="size-8 rounded-full"
        asChild
      >
        <Link
          to="/$"
          params={{ _splat: "configuracoes" }}
          aria-label="Preferências do workspace"
        >
          <Settings className="size-4 menu-icon-violet" />
        </Link>
      </Button>
    </header>
  );
}

function ConnectionGate({
  unauthorized,
  unreachable,
  error,
  onDemo,
  onRetry,
}: {
  unauthorized: boolean;
  unreachable: boolean;
  error: unknown;
  onDemo: () => void;
  onRetry: () => void;
}) {
  const queryClient = useQueryClient();
  const [token, setToken] = useState("");
  const hasToken = readToken().length > 0;
  function submit(event: FormEvent) {
    event.preventDefault();
    saveToken(token.trim());
    setToken("");
    void queryClient.invalidateQueries({ queryKey: ["snapshot"] });
  }
  return (
    <div className="mx-auto flex min-h-[70vh] max-w-5xl items-center justify-center">
      <div className="grid w-full overflow-hidden rounded-lg border border-border bg-surface lg:grid-cols-[1.05fr_.95fr]">
        <div className="border-b border-border p-7 lg:border-b-0 lg:border-r lg:p-10">
          <div className="mb-6 grid size-11 place-items-center rounded-md bg-primary text-primary-foreground">
            <LockKeyhole />
          </div>
          <h1 className="text-2xl font-semibold">Conectar ao Observa Lite</h1>
          <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
            Informe o token de administração do servidor existente. Ele ficará
            somente nesta aba e será enviado no cabeçalho das chamadas REST.
          </p>
          <div className="mt-5 inline-flex items-center gap-2 rounded-md border border-warning/30 bg-warning/10 px-3 py-2 text-xs text-warning">
            <KeyRound className="size-4" /> Acesso por token (modo inicial)
          </div>
          <form onSubmit={submit} className="mt-7 space-y-3">
            <Label htmlFor="api-token">Token da API</Label>
            <Input
              id="api-token"
              type="password"
              autoComplete="off"
              value={token}
              onChange={(event) => setToken(event.target.value)}
              placeholder="Cole o token do servidor"
              required
            />
            <div className="flex flex-wrap gap-2">
              <Button type="submit">Conectar</Button>
              {hasToken && (
                <Button
                  variant="outline"
                  type="button"
                  onClick={() => {
                    saveToken("");
                    onRetry();
                  }}
                >
                  Remover token
                </Button>
              )}
            </div>
          </form>
          {unauthorized && (
            <p className="mt-4 text-xs text-danger">
              O servidor respondeu 401. Confira o token e tente novamente.
            </p>
          )}
        </div>
        <div className="flex flex-col justify-between bg-surface-raised p-7 lg:p-10">
          <div>
            <div className="text-xs font-semibold uppercase text-muted-foreground">
              Estado da conexão
            </div>
            <div className="mt-4 flex items-start gap-3">
              <span
                className={cn(
                  "mt-1 size-2 rounded-full",
                  unauthorized ? "bg-danger" : "bg-warning",
                )}
              />
              <div>
                <div className="font-medium">
                  {unauthorized
                    ? "Autorização necessária"
                    : unreachable
                      ? "Servidor não alcançável"
                      : "Conexão pendente"}
                </div>
                <p className="mt-1 text-sm text-muted-foreground">
                  {error instanceof Error
                    ? error.message
                    : "Verifique o endereço e a disponibilidade do backend Go."}
                </p>
              </div>
            </div>
          </div>
          <div className="mt-10 border-t border-border pt-6">
            <p className="text-sm font-medium">
              Precisa avaliar a interface sem backend?
            </p>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              O modo demonstração usa dados ilustrativos e mantém uma faixa
              visível em todas as páginas.
            </p>
            <Button variant="outline" className="mt-4" onClick={onDemo}>
              Entrar no modo demonstração
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="space-y-5">
      <div className="h-8 w-64 animate-pulse rounded bg-muted" />
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {[1, 2, 3, 4].map((i) => (
          <div
            key={i}
            className="h-28 animate-pulse rounded-lg border border-border bg-surface"
          />
        ))}
      </div>
      <div className="h-[480px] animate-pulse rounded-lg border border-border bg-surface" />
    </div>
  );
}

function PageRouter({
  path: rawPath,
  snapshot,
  demo,
  globalSearch,
}: {
  path: string;
  snapshot: Snapshot;
  demo: boolean;
  globalSearch: string;
}) {
  const initialCreate = rawPath.endsWith("/novo");
  const path = initialCreate ? rawPath.slice(0, -5) : rawPath;
  if (path === "processos") return <ProcessExplorer snapshot={snapshot} />;
  if (path === "chaves-api") return <WorkspaceAccess />;
  if (path === "configuracoes") return <WorkspacePreferences />;
  if (path === "ajuda" || path.startsWith("ajuda/"))
    return (
      <HelpCenter key={path} path={path} snapshot={snapshot} demo={demo} />
    );
  if (/^traces\/[a-fA-F0-9]{32}$/.test(path))
    return (
      <APMPage
        key={path}
        path="traces"
        initialTrace={path.split("/")[1]!}
        demo={demo}
        globalSearch={globalSearch}
      />
    );
  if (["problemas-apm", "correlacao"].includes(path))
    return (
      <APMProblemsPage
        key={rawPath}
        path={path}
        demo={demo}
        initialCreate={initialCreate}
      />
    );
  if (path === "slos")
    return <SLOPage key={rawPath} demo={demo} initialCreate={initialCreate} />;
  if (["telemetria-logs", "metricas", "itens-metricas"].includes(path))
    return <TelemetryPage key={path} path={path} demo={demo} />;
  if (path === "dependencias")
    return (
      <MappingView kind="service" map={snapshot.service_map} demo={demo} />
    );
  if (
    ["servicos", "aplicacoes", "requisicoes", "erros", "traces"].includes(path)
  )
    return (
      <APMPage key={path} path={path} demo={demo} globalSearch={globalSearch} />
    );
  if (["service-mapping", "network-mapping", "process-mapping"].includes(path))
    return (
      <MappingView
        kind={
          path === "service-mapping"
            ? "service"
            : path === "process-mapping"
              ? "process"
              : "network"
        }
        map={
          path === "service-mapping"
            ? snapshot.service_map
            : path === "process-mapping"
              ? (snapshot.process_map ?? { nodes: [], edges: [] })
              : snapshot.network_map
        }
        demo={demo}
      />
    );
  if (path === "extensoes" || path === "minhas-extensoes")
    return <ExtensionHub configuredOnly={path === "minhas-extensoes"} />;
  if (
    path === "usuarios-acesso" ||
    path === "autenticacao" ||
    path === "chaves-api"
  )
    return <AuthSettings section={path} />;
  if (path === "templates")
    return (
      <MonitorManager
        key={rawPath}
        items={snapshot.templates}
        demo={demo}
        initialCreate={initialCreate}
      />
    );
  if (path === "data-sources")
    return (
      <DataSourceEditor
        key={rawPath}
        items={snapshot.datasources}
        demo={demo}
        initialCreate={initialCreate}
      />
    );
  if (path === "logs")
    return <LogsPage sources={snapshot.datasources} demo={demo} />;
  if (["dashboards", "editor-paineis", "html-graphics"].includes(path))
    return (
      <OperationalDashboards
        key={rawPath}
        items={snapshot.dashboards}
        demo={demo}
        initialCreate={initialCreate}
      />
    );
  if (["incidentes", "prioridades", "alertas"].includes(path))
    return <IncidentList events={snapshot.events} />;
  if (["agentes", "gateways", "host-groups", "network-zones"].includes(path))
    return <AgentRoutingPage snapshot={snapshot} path={path} />;
  if (
    path === "hosts" ||
    path === "processos" ||
    path === "containers" ||
    path === "kubernetes"
  )
    return <InfrastructurePage snapshot={snapshot} path={path} />;
  if (!path || path === "saude")
    return <Overview snapshot={snapshot} demo={demo} />;
  return <FuturePage title={labels.get(path) ?? "Área operacional"} />;
}

function AgentRoutingPage({
  snapshot,
  path,
}: {
  snapshot: Snapshot;
  path: string;
}) {
  const qc = useQueryClient();
  const gatewayQuery = useQuery({
    queryKey: ["gateways"],
    queryFn: api.gateways,
    enabled: path === "gateways",
    retry: false,
  });
  const [gatewayDraft, setGatewayDraft] = useState<GatewayEndpoint>({
    id: "",
    network_zone: "default",
    url: "",
    priority: 10,
    enabled: true,
  });
  const saveGateway = useMutation({
    mutationFn: api.saveGateway,
    onSuccess: () => {
      toast.success("Gateway registrado");
      void qc.invalidateQueries({ queryKey: ["gateways"] });
    },
    onError: (e) => toast.error(String(e)),
  });
  const removeGateway = useMutation({
    mutationFn: api.removeGateway,
    onSuccess: () => {
      toast.success("Gateway removido");
      void qc.invalidateQueries({ queryKey: ["gateways"] });
    },
    onError: (e) => toast.error(String(e)),
  });
  const hosts = snapshot.hosts;
  const groups = new Map<string, number>();
  const zones = new Map<string, number>();
  hosts.forEach((h) => {
    const g = h.host_group || "Sem grupo";
    const z = h.network_zone || "default";
    groups.set(g, (groups.get(g) || 0) + 1);
    zones.set(z, (zones.get(z) || 0) + 1);
  });
  const title =
    path === "agentes"
      ? "Agentes"
      : path === "gateways"
        ? "Gateways"
        : path === "host-groups"
          ? "Host Groups"
          : "Network Zones";
  const description =
    path === "gateways"
      ? "Endpoints HTTPS autorizados por zona, com prioridade e fallback opcional. Um registro não comprova conectividade ou saúde."
      : "Inventário observado a partir de agentes autenticados; grupos e zonas são informados na configuração da instalação.";
  if (path === "gateways")
    return (
      <>
        <PageHeader
          eyebrow="Administração / Conectividade"
          title={title}
          description={description}
        />
        <section className="rounded-lg border border-border bg-surface p-5">
          <h2 className="text-sm font-semibold">Cadastrar Gateway HTTPS</h2>
          <p className="mt-2 text-xs text-muted-foreground">
            Esta tela autoriza os destinos que o Agent pode utilizar. O Agent
            precisa habilitar o modo control-plane para consultar as rotas; o
            heartbeat dos Gateways não está implementado.
          </p>
          <form
            className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-5"
            onSubmit={(e) => {
              e.preventDefault();
              saveGateway.mutate(gatewayDraft);
            }}
          >
            <label className="space-y-1 text-xs">
              Identificador
              <Input
                required
                value={gatewayDraft.id}
                placeholder="gate-a"
                onChange={(e) =>
                  setGatewayDraft((v) => ({ ...v, id: e.target.value }))
                }
              />
            </label>
            <label className="space-y-1 text-xs">
              Network Zone
              <Input
                required
                value={gatewayDraft.network_zone}
                placeholder="zone-a"
                onChange={(e) =>
                  setGatewayDraft((v) => ({
                    ...v,
                    network_zone: e.target.value,
                  }))
                }
              />
            </label>
            <label className="space-y-1 text-xs xl:col-span-2">
              URL HTTPS
              <Input
                required
                type="url"
                value={gatewayDraft.url}
                placeholder="https://gateway.example:18443"
                onChange={(e) =>
                  setGatewayDraft((v) => ({ ...v, url: e.target.value }))
                }
              />
            </label>
            <label className="space-y-1 text-xs">
              Prioridade (menor = primeira)
              <Input
                required
                type="number"
                min={0}
                max={1000}
                value={gatewayDraft.priority}
                onChange={(e) =>
                  setGatewayDraft((v) => ({
                    ...v,
                    priority: Number(e.target.value),
                  }))
                }
              />
            </label>
            <label className="flex items-center gap-2 text-xs">
              <Switch
                checked={gatewayDraft.enabled}
                onCheckedChange={(enabled) =>
                  setGatewayDraft((v) => ({ ...v, enabled }))
                }
              />{" "}
              Rota habilitada
            </label>
            <Button
              className="max-w-52"
              type="submit"
              disabled={saveGateway.isPending}
            >
              <Plus className="mr-2 size-4" />
              Cadastrar
            </Button>
          </form>
        </section>
        <section className="mt-4 overflow-auto rounded-lg border border-border bg-surface">
          <div className="border-b border-border p-4 text-sm font-medium">
            Rotas cadastradas
          </div>
          {gatewayQuery.isError ? (
            <p className="p-5 text-sm text-destructive">
              Não foi possível consultar os Gateways autorizados.
            </p>
          ) : gatewayQuery.isLoading ? (
            <p className="p-5 text-sm text-muted-foreground">
              Carregando configuração...
            </p>
          ) : (
            <table className="w-full min-w-[650px] text-left text-sm">
              <thead className="border-b border-border text-xs text-muted-foreground">
                <tr>
                  <th className="p-3">ID</th>
                  <th className="p-3">Network Zone</th>
                  <th className="p-3">URL</th>
                  <th className="p-3">Prioridade</th>
                  <th className="p-3">Estado</th>
                  <th className="p-3">Ações</th>
                </tr>
              </thead>
              <tbody>
                {gatewayQuery.data?.gateways.map((g) => (
                  <tr key={g.id} className="border-b border-border/60">
                    <td className="p-3">{g.id}</td>
                    <td className="p-3">{g.network_zone}</td>
                    <td className="p-3 font-mono text-xs">{g.url}</td>
                    <td className="p-3">{g.priority}</td>
                    <td className="p-3">
                      {g.enabled ? "Autorizado" : "Desativado"}
                    </td>
                    <td className="p-3">
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={removeGateway.isPending}
                        onClick={() => {
                          if (
                            window.confirm(
                              "Excluir a rota de Gateway " + g.id + "?",
                            )
                          )
                            removeGateway.mutate(g.id);
                        }}
                      >
                        Excluir
                      </Button>
                    </td>
                  </tr>
                ))}
                {!gatewayQuery.data?.gateways.length && (
                  <tr>
                    <td
                      colSpan={6}
                      className="p-6 text-center text-muted-foreground"
                    >
                      Nenhum Gateway configurado
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          )}
        </section>
        <p className="mt-4 text-xs text-muted-foreground">
          Disponibilidade: não verificada. Configure o Gateway com TLS válido e
          faça os testes de conectividade antes de habilitar roteamento nos
          Agents.
        </p>
      </>
    );
  const rows =
    path === "host-groups"
      ? Array.from(groups).map(([name, count]) => [name, count])
      : path === "network-zones"
        ? Array.from(zones).map(([name, count]) => [name, count])
        : hosts.map((h) => [
            h.name,
            h.os,
            h.host_group || "Sem grupo",
            h.network_zone || "default",
            h.monitoring_mode || "legacy-discovery",
            new Date(h.last_seen).toLocaleString("pt-BR"),
          ]);
  return (
    <>
      <PageHeader
        eyebrow="Administração / Conectividade"
        title={title}
        description={description}
      />
      <div className="overflow-auto rounded-lg border border-border bg-surface">
        <table className="w-full min-w-[550px] text-left text-sm">
          <thead className="border-b border-border bg-muted/30 text-xs text-muted-foreground">
            <tr>
              {(path === "agentes"
                ? [
                    "Host",
                    "Sistema",
                    "Host Group",
                    "Network Zone",
                    "Modo",
                    "Última observação",
                  ]
                : [
                    path === "host-groups" ? "Host Group" : "Network Zone",
                    "Hosts",
                  ]
              ).map((t) => (
                <th className="px-4 py-3 font-medium" key={t}>
                  {t}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.length ? (
              rows.map((row, n) => (
                <tr key={n} className="border-b border-border/70">
                  {row.map((cell, j) => (
                    <td key={j} className="px-4 py-3">
                      {String(cell)}
                    </td>
                  ))}
                </tr>
              ))
            ) : (
              <tr>
                <td
                  colSpan={6}
                  className="p-8 text-center text-muted-foreground"
                >
                  Nenhum host observado neste escopo.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <p className="mt-4 text-xs text-muted-foreground">
        Network Zone é o agrupamento da conexão, não prova de roteamento por
        Gateway. Modos mais profundos ainda não estão disponíveis nesta versão.
      </p>
    </>
  );
}

function PageHeader({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow?: string;
  title: string;
  description: string;
  actions?: ReactNode;
}) {
  return (
    <div className="mb-6 flex flex-col justify-between gap-4 lg:flex-row lg:items-end">
      <div>
        {eyebrow && (
          <div className="mb-1 text-[11px] font-semibold uppercase text-primary">
            {eyebrow}
          </div>
        )}
        <h1 className="text-2xl font-semibold tracking-normal">{title}</h1>
        <p className="mt-1 max-w-3xl text-sm text-muted-foreground">
          {description}
        </p>
      </div>
      {actions && <div className="flex shrink-0 gap-2">{actions}</div>}
    </div>
  );
}
function KPIStats({
  items,
}: {
  items: Array<{
    label: string;
    value: string | number;
    detail: string;
    tone?: string;
    icon: LucideIcon;
  }>;
}) {
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      {items.map((item) => (
        <div
          key={item.label}
          className="rounded-lg border border-border bg-surface p-4"
        >
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-muted-foreground">
              {item.label}
            </span>
            <item.icon className={cn("size-4", item.tone ?? "text-primary")} />
          </div>
          <div className="mt-3 text-2xl font-semibold tabular-nums">
            {item.value}
          </div>
          <div className="mt-1 text-xs text-muted-foreground">
            {item.detail}
          </div>
        </div>
      ))}
    </div>
  );
}

function Overview({ snapshot, demo }: { snapshot: Snapshot; demo: boolean }) {
  const services = snapshot.service_map.nodes.filter(
    (node) => node.kind === "service",
  ).length;
  const issues = snapshot.events.filter(
    (event) => event.status === "open",
  ).length;
  return (
    <>
      <WorkspaceWelcome snapshot={snapshot} demo={demo} />
      <PageHeader
        eyebrow="Visão geral"
        title="Visão geral"
        description="Hosts, serviços, monitores e eventos do ambiente."
        actions={
          <span className="flex items-center gap-2 text-xs text-muted-foreground">
            <span className="size-2 rounded-full bg-success" />
            {demo
              ? "Amostra demonstrativa"
              : `Gerado ${formatDate(snapshot.generated_at)}`}
          </span>
        }
      />
      <KPIStats
        items={[
          {
            label: "Hosts observados",
            value: snapshot.hosts.length,
            detail: "Com inventário recente",
            icon: Server,
          },
          {
            label: "Serviços APM confirmados",
            value: services,
            detail: `${snapshot.process_map?.nodes.length ?? 0} entidades no Process Mapping`,
            icon: Workflow,
          },
          {
            label: "Prioridades abertas",
            value: issues,
            detail: "Requerem análise",
            icon: AlertTriangle,
            tone: issues ? "text-warning" : "text-success",
          },
          {
            label: "Monitores ativos",
            value: snapshot.templates.filter((item) => item.enabled).length,
            detail: `${snapshot.templates.length} templates configurados`,
            icon: HeartPulse,
          },
        ]}
      />
      <div className="mt-5 grid gap-5 xl:grid-cols-[1.45fr_.75fr]">
        <section className="rounded-lg border border-border bg-surface">
          <SectionTitle
            title="Prioridades agora"
            subtitle="Eventos ordenados por impacto operacional"
            link="/incidentes"
          />
          {snapshot.events.length ? (
            <IncidentRows events={snapshot.events.slice(0, 5)} />
          ) : (
            <EmptyState
              icon={ShieldCheck}
              title="Nenhuma prioridade aberta"
              description="Eventos do backend aparecerão aqui quando forem detectados."
            />
          )}
        </section>
        <section className="rounded-lg border border-border bg-surface">
          <SectionTitle
            title="Inventário recente"
            subtitle="Últimos hosts observados"
          />
          {snapshot.hosts.length ? (
            snapshot.hosts.slice(0, 5).map((host) => (
              <div
                key={host.id}
                className="flex items-center gap-3 border-t border-border px-4 py-3"
              >
                <div className="grid size-8 place-items-center rounded bg-muted">
                  <Server className="size-4" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium">
                    {host.name}
                  </div>
                  <div className="truncate text-xs text-muted-foreground">
                    {host.os} · {host.addresses.join(", ")}
                  </div>
                </div>
                <span className="size-2 rounded-full bg-success" />
              </div>
            ))
          ) : (
            <EmptyState
              icon={Server}
              title="Sem hosts"
              description="Instale ou conecte um agente para começar a descoberta."
            />
          )}
        </section>
      </div>
    </>
  );
}

function SectionTitle({
  title,
  subtitle,
  link,
}: {
  title: string;
  subtitle: string;
  link?: string;
}) {
  return (
    <div className="flex items-center justify-between p-4">
      <div>
        <h2 className="text-sm font-semibold">{title}</h2>
        <p className="mt-0.5 text-xs text-muted-foreground">{subtitle}</p>
      </div>
      {link && (
        <Link
          to="/$"
          params={{ _splat: link.replace(/^\//, "") }}
          className="text-xs font-medium text-primary hover:underline"
        >
          Ver tudo
        </Link>
      )}
    </div>
  );
}
function EmptyState({
  icon: Icon,
  title,
  description,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
}) {
  return (
    <div className="flex min-h-52 flex-col items-center justify-center p-8 text-center">
      <div className="grid size-11 place-items-center rounded-lg border border-border bg-muted text-muted-foreground">
        <Icon className="size-5" />
      </div>
      <h3 className="mt-4 text-sm font-semibold">{title}</h3>
      <p className="mt-1 max-w-sm text-xs leading-5 text-muted-foreground">
        {description}
      </p>
    </div>
  );
}

function IncidentRows({ events }: { events: Snapshot["events"] }) {
  return (
    <div>
      {events.map((event) => (
        <div
          key={event.id}
          className="grid gap-3 border-t border-border px-4 py-3 sm:grid-cols-[86px_1fr_120px] sm:items-center"
        >
          <Severity value={event.severity} />
          <div className="min-w-0">
            <div className="truncate text-sm font-medium">{event.summary}</div>
            <div className="mt-0.5 truncate text-xs text-muted-foreground">
              {event.entity} · {event.evidence[0] ?? "Sem evidência adicional"}
            </div>
          </div>
          <div className="text-left text-xs text-muted-foreground sm:text-right">
            {formatDate(event.at)}
          </div>
        </div>
      ))}
    </div>
  );
}
function Severity({ value }: { value: string }) {
  const severe = ["alta", "crítica", "critical"].includes(value.toLowerCase());
  return (
    <span
      className={cn(
        "inline-flex w-fit items-center rounded-sm border px-2 py-1 text-[10px] font-semibold uppercase",
        severe
          ? "border-danger/30 bg-danger/10 text-danger"
          : "border-warning/30 bg-warning/10 text-warning",
      )}
    >
      {value}
    </span>
  );
}

function IncidentList({ events }: { events: Snapshot["events"] }) {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const filtered = events.filter(
    (e) =>
      (status === "all" || e.status === status) &&
      `${e.summary} ${e.entity} ${e.severity}`
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  return (
    <>
      <PageHeader
        eyebrow="Operação"
        title="Eventos de monitores"
        description="Investigue verificações, entidades afetadas e evidências coletadas."
        actions={
          <Button
            variant="outline"
            onClick={() => downloadJSON("observa-eventos.json", filtered)}
          >
            Exportar resultados
          </Button>
        }
      />
      <div className="mb-4 flex flex-wrap items-center gap-3">
        <Input
          aria-label="Buscar eventos"
          placeholder="Buscar evento ou entidade…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
        <select
          aria-label="Estado dos eventos"
          className="h-10 rounded-lg border bg-surface px-3 text-sm"
          value={status}
          onChange={(e) => setStatus(e.target.value)}
        >
          <option value="all">Todos os estados</option>
          <option value="open">Abertos</option>
          <option value="resolved">Resolvidos</option>
        </select>
        <span className="text-xs text-muted-foreground">
          {filtered.length} resultados
        </span>
      </div>
      <section className="rounded-xl border border-border bg-surface">
        {filtered.length ? (
          <IncidentRows events={filtered} />
        ) : (
          <EmptyState
            icon={ShieldCheck}
            title="Nenhum evento encontrado"
            description="Ajuste os filtros ou configure um monitor HTTP/TCP."
          />
        )}
      </section>
    </>
  );
}

function ServicesPage({
  snapshot,
  filter,
}: {
  snapshot: Snapshot;
  filter: string;
}) {
  const nodes = snapshot.service_map.nodes.filter((n) =>
    n.name.toLowerCase().includes(filter.toLowerCase()),
  );
  return (
    <>
      <PageHeader
        eyebrow="Aplicações / APM"
        title="Serviços"
        description="Serviços descobertos, runtime, instâncias e relações observadas."
        actions={
          <Button asChild>
            <Link to="/$" params={{ _splat: "service-mapping" }}>
              <Workflow /> Abrir mapa
            </Link>
          </Button>
        }
      />
      <div className="overflow-hidden rounded-lg border border-border bg-surface">
        <DataTable
          headers={[
            "Serviço",
            "Tipo",
            "Runtime / host",
            "Instâncias",
            "Estado",
          ]}
        >
          {nodes.map((node) => (
            <tr key={node.id} className="border-t border-border">
              <Cell strong>{node.name}</Cell>
              <Cell>{node.kind}</Cell>
              <Cell>{node.runtime ?? node.host ?? "—"}</Cell>
              <Cell>{node.instances ?? 1}</Cell>
              <Cell>
                <Status value={node.status} />
              </Cell>
            </tr>
          ))}
        </DataTable>
        {!nodes.length && (
          <EmptyState
            icon={Server}
            title="Nenhum serviço encontrado"
            description="A descoberta será exibida quando o snapshot trouxer nós de serviço."
          />
        )}
      </div>
    </>
  );
}
function InfrastructurePage({
  snapshot,
  path,
}: {
  snapshot: Snapshot;
  path: string;
}) {
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<Snapshot["hosts"][number] | null>(
    null,
  );
  const hosts = snapshot.hosts.filter((h) =>
    `${h.name} ${h.os} ${h.addresses.join(" ")}`
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  const processes = snapshot.hosts
    .flatMap((h) => h.processes.map((p) => ({ ...p, host: h.name })))
    .filter((p) =>
      `${p.name} ${p.service} ${p.runtime} ${p.host} ${p.pid}`
        .toLowerCase()
        .includes(search.toLowerCase()),
    );
  if (path !== "hosts" && path !== "processos")
    return <FuturePage title={labels.get(path) ?? "Infraestrutura"} />;
  return (
    <>
      <PageHeader
        eyebrow="Infraestrutura"
        title={path === "hosts" ? "Hosts" : "Processos"}
        description="Explore hosts, processos e runtimes observados pelos agentes."
        actions={
          <Button
            variant="outline"
            onClick={() =>
              downloadJSON(
                `observa-${path}.json`,
                path === "hosts" ? hosts : processes,
              )
            }
          >
            Exportar resultados
          </Button>
        }
      />{" "}
      <div className="mb-4 flex items-center gap-3">
        <Input
          aria-label="Buscar inventário"
          placeholder={
            path === "hosts"
              ? "Buscar host, IP ou sistema…"
              : "Buscar processo, serviço, runtime ou PID…"
          }
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-md"
        />
        <span className="text-xs text-muted-foreground">
          {path === "hosts" ? hosts.length : processes.length} resultados
        </span>
      </div>
      <div className="overflow-hidden rounded-lg border border-border bg-surface">
        {path === "hosts" ? (
          <DataTable
            headers={[
              "Host",
              "Sistema operacional",
              "Endereços",
              "Processos",
              "Última observação",
            ]}
          >
            {hosts.map((host) => (
              <tr key={host.id} className="border-t border-border">
                <Cell strong>
                  <button
                    className="font-medium text-primary hover:underline"
                    onClick={() => setSelected(host)}
                  >
                    {host.name}
                  </button>
                </Cell>
                <Cell>{host.os}</Cell>
                <Cell>{host.addresses.join(", ") || "—"}</Cell>
                <Cell>{applicationProcesses(host).length}</Cell>
                <Cell>{formatDate(host.last_seen)}</Cell>
              </tr>
            ))}
          </DataTable>
        ) : (
          <DataTable
            headers={["PID", "Processo", "Serviço", "Runtime", "Host", "RSS"]}
          >
            {processes.map((p) => (
              <tr key={`${p.host}-${p.pid}`} className="border-t border-border">
                <Cell>{p.pid}</Cell>
                <Cell strong>{p.name}</Cell>
                <Cell>{p.service}</Cell>
                <Cell>{p.runtime}</Cell>
                <Cell>{p.host}</Cell>
                <Cell>{formatBytes(p.rss_bytes)}</Cell>
              </tr>
            ))}
          </DataTable>
        )}
      </div>
      <Sheet
        open={selected !== null}
        onOpenChange={(open) => {
          if (!open) setSelected(null);
        }}
      >
        <SheetContent className="overflow-y-auto sm:max-w-xl">
          <SheetHeader>
            <SheetTitle>{selected?.name}</SheetTitle>
            <SheetDescription>
              Inventário observado pelo agente
            </SheetDescription>
          </SheetHeader>
          {selected && (
            <div className="space-y-5 p-5">
              <dl className="grid grid-cols-2 gap-4 text-sm">
                {[
                  ["Sistema", selected.os],
                  ["Última coleta", formatDate(selected.last_seen)],
                  ["Host Group", selected.host_group || "Não definido"],
                  ["Network Zone", selected.network_zone || "Não definida"],
                  ["Modo", selected.monitoring_mode || "Não informado"],
                  [
                    "Processos de aplicação",
                    String(applicationProcesses(selected).length),
                  ],
                ].map(([label, value]) => (
                  <div key={label}>
                    <dt className="text-xs text-muted-foreground">{label}</dt>
                    <dd className="mt-1 break-words font-medium">{value}</dd>
                  </div>
                ))}
              </dl>
              <div className="rounded-xl border p-4">
                <h3 className="text-sm font-semibold">Endereços</h3>
                <p className="mt-2 break-all text-xs text-muted-foreground">
                  {selected.addresses.join(", ") || "Não informados"}
                </p>
              </div>
              <Button
                variant="outline"
                onClick={() => downloadJSON("observa-host.json", selected)}
              >
                Exportar inventário deste host
              </Button>
              <h3 className="text-sm font-semibold">Processos observados</h3>
              {applicationProcesses(selected).map((p) => (
                <div
                  key={p.pid}
                  className="flex items-center justify-between border-b py-2 text-xs"
                >
                  <div className="min-w-0">
                    <p className="truncate font-medium">{p.name}</p>
                    <p className="text-muted-foreground">
                      PID {p.pid} · {p.runtime || "Runtime não identificado"}
                    </p>
                  </div>
                  <span>{formatBytes(p.rss_bytes)}</span>
                </div>
              ))}
            </div>
          )}
        </SheetContent>
      </Sheet>
    </>
  );
}

function DataTable({
  headers,
  children,
}: {
  headers: string[];
  children: ReactNode;
}) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[720px] text-left text-xs">
        <thead className="bg-muted/40 text-muted-foreground">
          <tr>
            {headers.map((header) => (
              <th key={header} className="px-4 py-3 font-medium">
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  );
}
function Cell({ children, strong }: { children: ReactNode; strong?: boolean }) {
  return (
    <td
      className={cn(
        "px-4 py-3 text-muted-foreground",
        strong && "font-medium text-foreground",
      )}
    >
      {children}
    </td>
  );
}
function Status({ value }: { value: string }) {
  const ok = ["ok", "healthy", "ativo"].includes(value.toLowerCase());
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 text-xs",
        ok ? "text-success" : "text-warning",
      )}
    >
      <span className="size-1.5 rounded-full bg-current" />
      {value}
    </span>
  );
}

type FlowData = MapNode & Record<string, unknown> & { layout?: TopologyLayout };
const TopologyNode = memo(({ data }: NodeProps<Node<FlowData>>) => {
  const Icon =
    data.kind === "database"
      ? Database
      : data.kind === "messaging"
        ? Radio
        : data.kind === "external"
          ? Globe2
          : Server;
  return (
    <div className="min-w-44 max-w-60 rounded-md border border-map-node-border bg-map-node p-3 shadow-map">
      <Handle
        type="target"
        position={data.layout === "vertical" ? Position.Top : Position.Left}
        className="!bg-primary"
      />
      <div className="flex items-center gap-2">
        <Icon className="size-4 shrink-0 text-primary" />
        <span
          title={data.status === "unknown" ? "Saúde não avaliada" : data.status}
          className={cn(
            "size-2 shrink-0 rounded-full",
            data.status === "ok"
              ? "bg-success"
              : data.status === "warning"
                ? "bg-warning"
                : data.status === "error" || data.status === "critical"
                  ? "bg-danger"
                  : "bg-muted-foreground",
          )}
        />
        <div className="break-words text-xs font-semibold">{data.name}</div>
      </div>
      <div className="mt-1.5 text-[10px] text-muted-foreground">
        {data.kind}
        {data.runtime ? ` · ${data.runtime}` : ""}
      </div>
      <Handle
        type="source"
        position={data.layout === "vertical" ? Position.Bottom : Position.Right}
        className="!bg-primary"
      />
    </div>
  );
});
TopologyNode.displayName = "TopologyNode";
const topologyNodeTypes = { topology: TopologyNode };
function MappingView({
  kind,
  map,
  demo,
}: {
  kind: "service" | "network" | "process";
  map: TopologyMap;
  demo: boolean;
}) {
  const [selectedID, setSelectedID] = useState(""),
    [layout, setLayout] = useState<TopologyLayout>("horizontal"),
    [search, setSearch] = useState(""),
    [nodeKind, setNodeKind] = useState(""),
    [hideIsolated, setHideIsolated] = useState(false);
  const filtered = useMemo(
    () => filterTopology(map, { search, kind: nodeKind, hideIsolated }),
    [map, search, nodeKind, hideIsolated],
  );
  const kinds = useMemo(
    () => [...new Set(map.nodes.map((node) => node.kind))].sort(),
    [map.nodes],
  );
  const nodes: Node<FlowData>[] = useMemo(() => {
    const positions = layoutTopology(filtered, layout);
    return filtered.nodes.map((node) => ({
      id: node.id,
      type: "topology",
      position: positions.get(node.id)!,
      data: { ...node, layout },
    }));
  }, [filtered, layout]);
  const edges: Edge[] = useMemo(
    () =>
      filtered.edges.map((edge) => ({
        id: edge.id,
        source: edge.from,
        target: edge.to,
        label: `${edge.kind} · ${edge.count}`,
        style: { strokeWidth: 1.5 },
        labelStyle: { fontSize: 10 },
      })),
    [filtered.edges],
  );
  const selected = filtered.nodes.find((node) => node.id === selectedID);
  const selectedEdges = selected
    ? filtered.edges.filter(
        (edge) => edge.from === selected.id || edge.to === selected.id,
      )
    : [];
  return (
    <>
      <PageHeader
        eyebrow={kind === "service" ? "Aplicações / APM" : "Infraestrutura"}
        title={
          kind === "service"
            ? "Service Mapping"
            : kind === "process"
              ? "Process Mapping"
              : "Network Mapping"
        }
        description={
          kind === "service"
            ? "Serviços e dependências observados em spans OpenTelemetry. A topologia não determina a saúde nem a causa raiz."
            : kind === "process"
              ? "Agrupamentos inferidos de processos Linux/Windows, sem garantia de vínculo APM."
              : "Fluxos IP observados; topologia física SNMP não disponível."
        }
      />
      <div className="mb-4 flex flex-wrap items-end gap-3 rounded border border-border bg-surface p-4">
        <label className="grid flex-1 gap-2 text-xs">
          Pesquisar nó
          <Input
            placeholder="Nome ou ID"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </label>
        <label className="grid gap-2 text-xs">
          Tipo
          <select
            className="rounded border border-input bg-background p-2"
            value={nodeKind}
            onChange={(event) => setNodeKind(event.target.value)}
          >
            <option value="">Todos</option>
            {kinds.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </label>
        <label className="grid gap-2 text-xs">
          Layout
          <select
            className="rounded border border-input bg-background p-2"
            value={layout}
            onChange={(event) =>
              setLayout(event.target.value as TopologyLayout)
            }
          >
            <option value="grid">Grade</option>
            <option value="horizontal">Horizontal</option>
            <option value="vertical">Vertical</option>
          </select>
        </label>
        <label className="flex items-center gap-2 py-2 text-xs">
          <input
            type="checkbox"
            checked={hideIsolated}
            onChange={(event) => setHideIsolated(event.target.checked)}
          />
          Ocultar isolados
        </label>
      </div>
      <p className="mb-3 text-xs text-muted-foreground">
        {filtered.nodes.length} nós · {filtered.edges.length} relações exibidas
        · {map.nodes.length} nós e {map.edges.length} relações recebidas
        {demo ? " · Amostra demonstrativa" : ""} · Cinza: saúde não avaliada
      </p>
      {filtered.truncated && (
        <p
          role="status"
          className="mb-3 rounded border border-warning/40 p-3 text-sm text-warning"
        >
          Exibindo {filtered.limit} de {filtered.matchedNodes} nós após os
          filtros. Refine a pesquisa para investigar outros nós.
        </p>
      )}
      <div className="grid min-h-[610px] overflow-hidden rounded-lg border border-border bg-surface lg:grid-cols-[1fr_310px]">
        <div className="relative min-h-[460px] border-b border-border lg:border-b-0 lg:border-r">
          {nodes.length ? (
            <ReactFlow
              key={layout + ":" + search + ":" + nodeKind + ":" + hideIsolated}
              nodes={nodes}
              edges={edges}
              nodeTypes={topologyNodeTypes}
              fitView
              minZoom={0.1}
              maxZoom={2}
              onNodeClick={(_, node) => setSelectedID(node.id)}
              colorMode="dark"
            >
              <Background gap={24} size={1} />
              <Controls />
              <MiniMap pannable zoomable />
            </ReactFlow>
          ) : (
            <EmptyState
              icon={Workflow}
              title="Nenhum nó encontrado"
              description={
                map.nodes.length
                  ? "Ajuste a pesquisa, o tipo ou a opção de ocultar isolados."
                  : kind === "service"
                    ? "Envie spans para o receptor OpenTelemetry. Processos e portas não representam serviços APM."
                    : "Os agentes precisam reportar processos e conexões para construir este mapa."
              }
            />
          )}
        </div>
        <aside className="p-5">
          {selected ? (
            <>
              <div className="flex items-start justify-between">
                <div>
                  <p className="text-[10px] font-semibold uppercase text-primary">
                    {selected.kind}
                  </p>
                  <h2 className="mt-1 break-all font-semibold">
                    {selected.name}
                  </h2>
                </div>
                <Button
                  aria-label="Fechar detalhes do nó"
                  variant="ghost"
                  size="icon"
                  className="size-7"
                  onClick={() => setSelectedID("")}
                >
                  <X />
                </Button>
              </div>
              <p className="mt-2 break-all font-mono text-[10px] text-muted-foreground">
                {selected.id}
              </p>
              <div className="mt-5 space-y-3 text-xs">
                <Detail
                  label="Estado"
                  value={
                    selected.status === "unknown"
                      ? "Não avaliado"
                      : selected.status
                  }
                />
                <Detail
                  label="Runtime"
                  value={selected.runtime ?? "Não informado"}
                />
                <Detail label="Host" value={selected.host ?? "Não informado"} />
                <Detail
                  label="Instâncias"
                  value={
                    selected.instances === undefined
                      ? "Não informado"
                      : String(selected.instances)
                  }
                />
              </div>
              <div className="mt-6 border-t border-border pt-5">
                <h3 className="text-xs font-semibold">
                  Evidências relacionadas no mapa exibido
                </h3>
                {selectedEdges.length ? (
                  selectedEdges.map((edge) => (
                    <div
                      key={edge.id}
                      className="mt-3 rounded border border-border bg-muted/30 p-3"
                    >
                      <p className="text-xs font-medium">
                        {edge.kind} · {edge.count} observações
                      </p>
                      <p className="mt-1 break-all text-[11px] leading-5 text-muted-foreground">
                        {edge.evidence || "Evidência não informada"}
                      </p>
                    </div>
                  ))
                ) : (
                  <p className="mt-2 text-xs text-muted-foreground">
                    Nenhuma relação no recorte exibido.
                  </p>
                )}
              </div>
            </>
          ) : (
            <EmptyState
              icon={PanelRight}
              title="Selecione um nó"
              description="Inspecione atributos recebidos e evidências das relações observadas."
            />
          )}
        </aside>
      </div>
    </>
  );
}

function Detail({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-3 border-b border-border pb-2">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium">{value}</span>
    </div>
  );
}

function DataSourceEditor({
  items,
  demo,
  initialCreate = false,
}: {
  items: DataSource[];
  demo: boolean;
  initialCreate?: boolean;
}) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(initialCreate && !demo);
  const [editing, setEditing] = useState<DataSource | null>(null);
  const create = useMutation({
    mutationFn: api.createDataSource,
    onSuccess: () => {
      toast.success("Fonte registrada no servidor");
      setOpen(false);
      void queryClient.invalidateQueries({ queryKey: ["snapshot"] });
    },
    onError: (error) => toast.error(error.message),
  });
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (demo) {
      toast.info("A gravação não está disponível no modo demonstração");
      return;
    }
    const data = new FormData(event.currentTarget);
    create.mutate({
      id: editing?.id ?? crypto.randomUUID(),
      name: String(data.get("name")),
      kind: String(data.get("kind")) as "prometheus" | "json" | "loki",
      url: String(data.get("url")),
      ...(String(data.get("secret_env"))
        ? { secret_env: String(data.get("secret_env")) }
        : {}),
    });
  }
  return (
    <>
      <PageHeader
        eyebrow="Explorar"
        title="Data Sources"
        description="Fontes Prometheus, JSON e Loki configuradas no backend. Segredos permanecem no servidor por variável de ambiente."
        actions={
          <Button
            disabled={demo}
            onClick={() => {
              setEditing(null);
              setOpen(true);
            }}
          >
            <Plus /> Nova fonte
          </Button>
        }
      />
      <div className="grid gap-3 lg:grid-cols-2">
        {items.map((item) => (
          <div
            key={item.id}
            className="rounded-lg border border-border bg-surface p-5"
          >
            <div className="flex items-start justify-between">
              <div className="flex gap-3">
                <div className="grid size-9 place-items-center rounded bg-muted">
                  <Database className="size-4 text-primary" />
                </div>
                <div>
                  <h2 className="text-sm font-semibold">{item.name}</h2>
                  <p className="mt-0.5 text-xs uppercase text-muted-foreground">
                    {item.kind}
                  </p>
                </div>
              </div>
              <span className="rounded-sm border border-success/30 bg-success/10 px-2 py-1 text-[10px] text-success">
                CONFIGURADA
              </span>
            </div>
            <p className="mt-5 truncate font-mono text-xs text-muted-foreground">
              {item.url}
            </p>
            <div className="mt-4 border-t border-border pt-3 text-xs text-muted-foreground">
              Credencial:{" "}
              {item.secret_env
                ? `variável ${item.secret_env}`
                : "não informada"}
            </div>
            <div className="mt-4 flex gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={demo}
                onClick={() => {
                  setEditing(item);
                  setOpen(true);
                }}
              >
                <Pencil className="size-3.5" />
                Editar fonte
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => downloadJSON("observa-fonte.json", item)}
              >
                Exportar definição
              </Button>
            </div>
          </div>
        ))}
        {!items.length && (
          <div className="lg:col-span-2">
            <EmptyState
              icon={Database}
              title="Nenhuma fonte configurada"
              description="Adicione uma fonte Prometheus ou JSON suportada pelo backend."
            />
          </div>
        )}
      </div>
      <EditorDialog
        open={open}
        setOpen={setOpen}
        title={editing ? "Editar fonte de dados" : "Nova fonte de dados"}
        description="Não insira segredos. Informe somente o nome da variável existente no servidor."
      >
        <form
          key={editing?.id ?? "new"}
          onSubmit={submit}
          className="space-y-4"
        >
          <Field
            name="name"
            label="Nome"
            placeholder="Prometheus principal"
            defaultValue={editing?.name}
          />
          <SelectField
            name="kind"
            label="Tipo"
            options={["prometheus", "json", "loki"]}
            defaultValue={editing?.kind}
          />
          <Field
            name="url"
            label="URL"
            type="url"
            placeholder="http://prometheus:9090"
            defaultValue={editing?.url}
          />
          <Field
            name="secret_env"
            label="Variável de segredo (opcional)"
            placeholder="PROMETHEUS_TOKEN"
            defaultValue={editing?.secret_env}
            required={false}
          />
          <Button className="w-full" disabled={create.isPending}>
            Salvar no servidor
          </Button>
        </form>
      </EditorDialog>
    </>
  );
}

function DashboardBuilder({
  items,
  demo,
}: {
  items: Dashboard[];
  demo: boolean;
}) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const create = useMutation({
    mutationFn: api.createDashboard,
    onSuccess: () => {
      toast.success("Dashboard salvo");
      setOpen(false);
      void queryClient.invalidateQueries({ queryKey: ["snapshot"] });
    },
    onError: (error) => toast.error(error.message),
  });
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (demo) {
      toast.info("A gravação não está disponível no modo demonstração");
      return;
    }
    const data = new FormData(event.currentTarget);
    create.mutate({
      id: crypto.randomUUID(),
      title: String(data.get("title")),
      panels: [
        {
          id: crypto.randomUUID(),
          title: String(data.get("panelTitle")),
          kind: String(data.get("kind")),
          ...(String(data.get("html"))
            ? { html: String(data.get("html")) }
            : {}),
          ...(String(data.get("query"))
            ? { query: String(data.get("query")) }
            : {}),
        },
      ],
    });
  }
  return (
    <>
      <PageHeader
        eyebrow="Visualização"
        title="Dashboards"
        description="Painéis salvos pelo backend, incluindo estatísticas e HTML isolado em sandbox."
        actions={
          <Button onClick={() => setOpen(true)}>
            <Plus /> Novo dashboard
          </Button>
        }
      />
      {items.length ? (
        <div className="grid gap-4 xl:grid-cols-2">
          {items.map((dashboard) => (
            <section
              key={dashboard.id}
              className="rounded-lg border border-border bg-surface p-4"
            >
              <h2 className="text-sm font-semibold">{dashboard.title}</h2>
              <div className="mt-4 grid gap-3 sm:grid-cols-2">
                {dashboard.panels.map((panel) => (
                  <div
                    key={panel.id}
                    className="min-h-36 rounded-md border border-border bg-surface-raised p-3"
                  >
                    <div className="text-xs font-medium text-muted-foreground">
                      {panel.title}
                    </div>
                    {panel.kind === "html" && panel.html ? (
                      <iframe
                        title={panel.title}
                        sandbox=""
                        srcDoc={`<style>${panel.css ?? ""}</style>${panel.html}`}
                        className="mt-3 h-28 w-full border-0 bg-transparent"
                      />
                    ) : (
                      <div className="mt-7 text-center text-3xl font-semibold text-primary">
                        {panel.query ?? "—"}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </section>
          ))}
        </div>
      ) : (
        <EmptyState
          icon={LayoutDashboard}
          title="Nenhum dashboard"
          description="Crie um dashboard e salve a definição no backend."
        />
      )}
      <EditorDialog
        open={open}
        setOpen={setOpen}
        title="Novo dashboard"
        description="Crie um primeiro painel stat ou HTML."
      >
        <form onSubmit={submit} className="space-y-4">
          <Field
            name="title"
            label="Título do dashboard"
            placeholder="Operação Linux"
          />
          <Field
            name="panelTitle"
            label="Título do painel"
            placeholder="Disponibilidade"
          />
          <SelectField name="kind" label="Tipo" options={["stat", "html"]} />
          <Field
            name="query"
            label="Consulta ou valor"
            placeholder={'up{job="api"}'}
          />
          <div>
            <Label htmlFor="html">HTML (opcional)</Label>
            <Textarea
              id="html"
              name="html"
              className="mt-2 min-h-28 font-mono text-xs"
              placeholder="<strong>Estado operacional</strong>"
            />
          </div>
          <Button className="w-full" disabled={create.isPending}>
            Salvar no servidor
          </Button>
        </form>
      </EditorDialog>
    </>
  );
}

type HubEntry = {
  id: string;
  name: string;
  category: string;
  description: string;
  available: boolean;
  configured: boolean;
  integration: string;
  version: string;
  icon: LucideIcon;
};
function ExtensionHub({ configuredOnly }: { configuredOnly: boolean }) {
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("Todos");
  const [selected, setSelected] = useState<HubEntry | null>(null);
  const query = useQuery({
    queryKey: ["extensions"],
    queryFn: api.extensions,
    retry: false,
    refetchInterval: 60_000,
  });
  const all: HubEntry[] = (query.data?.extensions ?? []).map((x) => ({
    ...x,
    icon:
      extensionManifest.find(
        (v) => v.name.toLowerCase() === x.name.toLowerCase(),
      )?.icon ?? Boxes,
  }));
  const categories = [
    "Todos",
    ...Array.from(new Set(all.map((x) => x.category))),
  ];
  const filtered = all.filter(
    (x) =>
      (!configuredOnly || x.configured) &&
      (category === "Todos" || category === x.category) &&
      (x.name + " " + x.description + " " + x.category)
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  return (
    <>
      <PageHeader
        eyebrow="Extensões"
        title={configuredOnly ? "Minhas extensões" : "Hub de Extensões"}
        description="Estado real consultado pelo backend: configurada, disponível ou planejada."
      />
      <section className="mb-4 rounded-lg border border-border bg-surface p-4">
        <div className="relative max-w-xl">
          <Search className="absolute left-3 top-2.5 size-4 text-muted-foreground" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
            placeholder="Buscar extensões, tecnologias e categorias"
          />
        </div>
        <div className="mt-3 flex flex-wrap gap-2">
          {categories.map((c) => (
            <Button
              key={c}
              size="sm"
              variant={category === c ? "default" : "outline"}
              onClick={() => setCategory(c)}
            >
              {c}
            </Button>
          ))}
        </div>
      </section>
      {query.isLoading ? (
        <LoadingState />
      ) : query.isError ? (
        <div className="rounded-lg border border-warning/30 p-4 text-warning">
          Não foi possível carregar o catálogo real. Verifique a API e o token
          de acesso.
        </div>
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={Boxes}
          title="Nenhuma extensão encontrada"
          description={
            configuredOnly
              ? "Nenhuma integração foi confirmada como configurada pelo servidor."
              : "Ajuste os filtros da pesquisa."
          }
        />
      ) : (
        <div className="grid gap-3 md:grid-cols-2 2xl:grid-cols-3">
          {filtered.map((item) => {
            const Icon = item.icon;
            return (
              <button
                key={item.id}
                onClick={() => setSelected(item)}
                className="rounded-lg border border-border bg-surface p-4 text-left transition-colors hover:border-primary/50 hover:bg-surface-raised"
              >
                <div className="flex items-start gap-3">
                  <div className="grid size-10 shrink-0 place-items-center rounded-md border border-border bg-muted text-primary">
                    <Icon className="size-5" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-start justify-between gap-2">
                      <h2 className="font-semibold">{item.name}</h2>
                      <span
                        className={cn(
                          "rounded-sm border px-1.5 py-0.5 text-[9px] uppercase",
                          item.configured
                            ? "border-success/30 text-success"
                            : item.available
                              ? "border-primary/30 text-primary"
                              : "border-border text-muted-foreground",
                        )}
                      >
                        {item.configured
                          ? "Configurada"
                          : item.available
                            ? "Disponível"
                            : "Planejada"}
                      </span>
                    </div>
                    <p className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">
                      {item.description}
                    </p>
                  </div>
                </div>
                <div className="mt-4 flex items-center justify-between border-t border-border pt-3 text-[10px] text-muted-foreground">
                  <span>{item.category}</span>
                  <span>{item.version}</span>
                </div>
              </button>
            );
          })}
        </div>
      )}
      <ExtensionDetail item={selected} onClose={() => setSelected(null)} />
    </>
  );
}
function ExtensionDetail({
  item,
  onClose,
}: {
  item: HubEntry | null;
  onClose: () => void;
}) {
  const navigate = useNavigate();
  const destination =
    item?.integration === "datasource"
      ? "data-sources"
      : item?.integration === "template"
        ? "templates"
        : item?.integration === "mcp"
          ? "mcp-hub"
          : item?.integration === "otlp"
            ? "servicos"
            : "agentes";
  return (
    <Sheet
      open={Boolean(item)}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SheetContent className="w-full border-border bg-surface sm:max-w-lg">
        {item && (
          <>
            <SheetHeader>
              <SheetTitle>{item.name}</SheetTitle>
              <SheetDescription>{item.description}</SheetDescription>
            </SheetHeader>
            <div className="mt-6 space-y-3 text-sm">
              <Detail label="Categoria" value={item.category} />
              <Detail label="Versão" value={item.version} />
              <Detail
                label="Estado"
                value={
                  item.configured
                    ? "Configurada"
                    : item.available
                      ? "Disponível"
                      : "Planejada"
                }
              />
            </div>
            <div className="mt-6 rounded-md border border-border bg-muted/40 p-4 text-xs text-muted-foreground">
              {item.integration === "otlp"
                ? "Envie traces OpenTelemetry ao endpoint /v1/traces usando o token do agente. JSON, protobuf e gzip são aceitos. Os endpoints /v1/logs e /v1/metrics também recebem sinais OTLP."
                : item.available
                  ? "Esta capacidade é reconhecida pelo backend. Configure-a no módulo correspondente."
                  : "Este conector está no roteiro de desenvolvimento e ainda não pode ser ativado."}
            </div>
            <Button
              className="mt-5 w-full"
              disabled={!item.available}
              onClick={() => {
                onClose();
                void navigate({ to: "/$", params: { _splat: destination } });
              }}
            >
              {item.integration === "otlp"
                ? "Serviços"
                : item.configured
                  ? "Abrir"
                  : item.available
                    ? "Configurar"
                    : "Em desenvolvimento"}
            </Button>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}

function AuthSettings({ section }: { section: string }) {
  return (
    <>
      <PageHeader
        eyebrow="Administração"
        title={
          section === "usuarios-acesso"
            ? "Usuários & acesso"
            : section === "autenticacao"
              ? "Autenticação (SSO/MFA)"
              : "Chaves de API"
        }
        description="Estado honesto dos recursos de segurança disponíveis nesta versão."
      />
      <div className="mb-5 flex items-center gap-3 rounded-lg border border-warning/30 bg-warning/10 p-4">
        <KeyRound className="size-5 text-warning" />
        <div>
          <div className="text-sm font-semibold">
            Acesso por token (modo inicial)
          </div>
          <p className="mt-0.5 text-xs text-muted-foreground">
            O backend atual usa um token administrativo. Usuários, sessões,
            RBAC, OIDC e MFA não são aplicados pelo servidor.
          </p>
        </div>
      </div>
      <Tabs defaultValue="acesso">
        <TabsList>
          <TabsTrigger value="acesso">Acesso atual</TabsTrigger>
          <TabsTrigger value="oidc">OIDC / SSO</TabsTrigger>
          <TabsTrigger value="roles">Perfis e roles</TabsTrigger>
        </TabsList>
        <TabsContent value="acesso">
          <SecurityCard
            icon={KeyRound}
            title="Token administrativo"
            status="Em uso"
            description="Mantido somente em sessionStorage nesta aba. Não há persistência longa."
          />
        </TabsContent>
        <TabsContent value="oidc">
          <div className="grid gap-4 lg:grid-cols-2">
            <SecurityCard
              icon={ShieldCheck}
              title="SSO via OIDC"
              status="Não configurado nesta versão"
              description="Preparado para Keycloak, Authentik ou Azure Entra. Requer implementação no backend antes da ativação."
            />
            <SecurityCard
              icon={LockKeyhole}
              title="MFA"
              status="Não configurado nesta versão"
              description="A exigência deverá ser aplicada no provedor de identidade, nunca simulada apenas na interface."
            />
          </div>
          <div className="mt-4 rounded-lg border border-border bg-surface p-5">
            <h3 className="text-sm font-semibold">
              Parâmetros previstos para OIDC
            </h3>
            <div className="mt-4 grid gap-4 md:grid-cols-2">
              <Field
                name="issuer"
                label="Issuer URL"
                placeholder="https://idp.exemplo/realms/observa"
                disabled
              />
              <Field
                name="client"
                label="Client ID"
                placeholder="observa-lite"
                disabled
              />
              <Field
                name="callback"
                label="Callback URL"
                placeholder="https://observa.exemplo/auth/callback"
                disabled
              />
              <Field
                name="mfa"
                label="Política MFA no IdP"
                placeholder="Obrigatória para administradores"
                disabled
              />
            </div>
            <p className="mt-4 text-xs text-muted-foreground">
              Segredos de cliente não devem ser enviados ao navegador.
              Configure-os somente no backend quando houver suporte.
            </p>
          </div>
        </TabsContent>
        <TabsContent value="roles">
          <SecurityCard
            icon={Users}
            title="RBAC"
            status="Não configurado nesta versão"
            description="Perfis, permissões e auditoria dependerão da validação efetiva no backend."
          />
        </TabsContent>
      </Tabs>
    </>
  );
}
function SecurityCard({
  icon: Icon,
  title,
  status,
  description,
}: {
  icon: LucideIcon;
  title: string;
  status: string;
  description: string;
}) {
  return (
    <div className="rounded-lg border border-border bg-surface p-5">
      <div className="flex items-start justify-between gap-3">
        <div className="grid size-9 place-items-center rounded bg-muted">
          <Icon className="size-4 text-primary" />
        </div>
        <span
          className={cn(
            "rounded-sm border px-2 py-1 text-[10px]",
            status === "Em uso"
              ? "border-success/30 bg-success/10 text-success"
              : "border-border text-muted-foreground",
          )}
        >
          {status}
        </span>
      </div>
      <h2 className="mt-4 text-sm font-semibold">{title}</h2>
      <p className="mt-1 text-xs leading-5 text-muted-foreground">
        {description}
      </p>
      <Button className="mt-4" variant="outline" disabled={status !== "Em uso"}>
        Configurar
      </Button>
    </div>
  );
}

function FuturePage({ title }: { title: string }) {
  return (
    <>
      <PageHeader
        eyebrow="Observa Lite"
        title={title}
        description="Esta área está preparada para receber telemetria e recursos quando o backend publicar suporte."
      />
      <div className="rounded-lg border border-border bg-surface">
        <EmptyState
          icon={LifeBuoy}
          title="Sem dados disponíveis"
          description="Nenhuma capacidade é simulada. Conecte a integração ou aguarde o suporte desta versão do servidor."
        />
      </div>
    </>
  );
}

function EditorDialog({
  open,
  setOpen,
  title,
  description,
  children,
}: {
  open: boolean;
  setOpen: (open: boolean) => void;
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="border-border bg-surface">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {children}
      </DialogContent>
    </Dialog>
  );
}
function Field({
  name,
  label,
  placeholder,
  type = "text",
  disabled = false,
  defaultValue,
  required = true,
}: {
  name: string;
  label: string;
  placeholder: string;
  type?: string;
  disabled?: boolean;
  defaultValue?: string | undefined;
  required?: boolean;
}) {
  return (
    <div>
      <Label htmlFor={name}>{label}</Label>
      <Input
        id={name}
        name={name}
        type={type}
        placeholder={placeholder}
        disabled={disabled}
        className="mt-2"
        required={required && !disabled}
        defaultValue={defaultValue}
      />
    </div>
  );
}
function SelectField({
  name,
  label,
  options,
  defaultValue,
}: {
  name: string;
  label: string;
  options: string[];
  defaultValue?: string | undefined;
}) {
  return (
    <div>
      <Label>{label}</Label>
      <Select name={name} defaultValue={defaultValue ?? options[0]}>
        <SelectTrigger className="mt-2">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option} value={option}>
              {option.toUpperCase()}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
function formatDate(value: string) {
  if (!value) return "—";
  return new Intl.DateTimeFormat("pt-BR", {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(value));
}
function formatBytes(value: number) {
  return new Intl.NumberFormat("pt-BR", {
    style: "unit",
    unit: "megabyte",
    maximumFractionDigits: 0,
  }).format(value / 1_048_576);
}
