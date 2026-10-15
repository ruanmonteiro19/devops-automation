import { useEffect, useState } from "react";
import { Link } from "@tanstack/react-router";
import {
  ArrowUpRight,
  ArrowRight,
  Search,
  Plus,
  Sparkles,
  BookOpen,
  LifeBuoy,
  Palette,
  Sun,
  Moon,
  Check,
  Copy,
  Download,
  Activity,
  Server,
  Workflow,
  ShieldCheck,
  Command,
  X,
} from "lucide-react";
import { toast } from "sonner";
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
import {
  applyAppearance,
  loadAppearance,
  defaultAppearance,
  downloadJSON,
  menuIconColor,
  quickActions,
  workspaceDestinations,
  type Appearance,
} from "@/lib/workspace";
import { type Snapshot } from "@/lib/api";
import { cn } from "@/lib/utils";

export function WorkspaceTools({
  snapshot,
  demo,
  current,
}: {
  snapshot?: Snapshot | undefined;
  demo: boolean;
  current: string;
}) {
  const [command, setCommand] = useState(false),
    [create, setCreate] = useState(false),
    [help, setHelp] = useState(false),
    [search, setSearch] = useState("");
  useEffect(() => {
    applyAppearance(loadAppearance());
    const key = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setCommand((v) => !v);
      }
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  }, []);
  const destinations = [...quickActions, ...workspaceDestinations].filter(
    (item) =>
      `${item.title} ${item.description}`
        .toLocaleLowerCase()
        .includes(search.toLocaleLowerCase()),
  );
  const close = () => {
    setCommand(false);
    setCreate(false);
    setSearch("");
  };
  return (
    <>
      <div className="workspace-tools flex flex-wrap items-center justify-between gap-3 border-b border-border/70 px-4 py-3 lg:px-7">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <span
            className={cn(
              "size-2 rounded-full",
              snapshot ? "bg-success" : "bg-warning",
            )}
          />
          {demo
            ? "Ambiente demonstrativo"
            : snapshot
              ? "Servidor conectado"
              : "Conexão pendente"}
          <span className="mx-1 text-border">/</span>
          <span className="hidden sm:inline">
            {snapshot
              ? `${snapshot.hosts.length} hosts · ${snapshot.service_map.nodes.filter((n) => n.kind === "service").length} serviços`
              : "Servidor de observabilidade"}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setCommand(true)}
            className="gap-3"
          >
            <Search className="size-3.5" />
            <span>Ir para…</span>
            <kbd className="hidden rounded border px-1 text-[10px] sm:inline">
              Ctrl K
            </kbd>
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setHelp(true)}>
            <LifeBuoy className="size-4" />
            Ajuda
          </Button>
          <Button size="sm" onClick={() => setCreate(true)}>
            <Plus className="size-4" />
            Criar
          </Button>
        </div>
      </div>
      <Dialog
        open={command || create}
        onOpenChange={(open) => {
          if (!open) close();
        }}
      >
        <DialogContent className="max-w-2xl overflow-hidden p-0">
          <DialogHeader className="px-6 pt-6">
            <DialogTitle className="flex items-center gap-2">
              {create ? (
                <Plus className="text-primary" />
              ) : (
                <Command className="text-primary" />
              )}
              {create ? "Criar" : "Buscar páginas e ações"}
            </DialogTitle>
            <DialogDescription>
              {create
                ? "Selecione o item que deseja criar."
                : "Busque uma página ou abra um formulário de criação."}
            </DialogDescription>
          </DialogHeader>
          {!create && (
            <div className="px-6">
              <Input
                autoFocus
                aria-label="Buscar ações e páginas"
                placeholder="Buscar serviços, dashboards, ajuda…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
          )}
          <div className="max-h-[60vh] overflow-y-auto px-3 pb-4">
            {(create ? quickActions : destinations).map((item) => (
              <Link
                key={`${item.title}-${item.path}`}
                to="/$"
                params={{ _splat: item.path }}
                onClick={close}
                className="group flex items-center gap-4 rounded-xl px-3 py-3 transition hover:bg-accent focus-visible:outline focus-visible:outline-ring"
              >
                <div
                  className={cn(
                    "grid size-9 shrink-0 place-items-center rounded border border-border bg-muted/40",
                    menuIconColor(item.path),
                  )}
                >
                  {item.path.includes("novo") ? (
                    <Plus className="size-4" />
                  ) : (
                    <ArrowUpRight className="size-4" />
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-semibold">{item.title}</p>
                  <p className="text-xs text-muted-foreground">
                    {item.description}
                  </p>
                </div>
                <ArrowRight className="size-4 text-muted-foreground transition group-hover:translate-x-1" />
              </Link>
            ))}
            {!create && !destinations.length && (
              <p className="p-8 text-center text-sm text-muted-foreground">
                Nenhum resultado. Tente “dashboard”, “logs” ou “ajuda”.
              </p>
            )}
          </div>
        </DialogContent>
      </Dialog>
      <Sheet open={help} onOpenChange={setHelp}>
        <SheetContent className="overflow-y-auto sm:max-w-md">
          <SheetHeader>
            <SheetTitle>Ajuda</SheetTitle>
            <SheetDescription>
              Orientação para a tarefa que você está fazendo.
            </SheetDescription>
          </SheetHeader>
          <div className="space-y-5 p-5">
            <div className="rounded-xl border border-primary/20 bg-primary/10 p-4">
              <BookOpen className="mb-3 text-primary" />
              <h3 className="font-semibold">Ajuda da página</h3>
              <p className="mt-2 text-sm text-muted-foreground">
                Use Criar para abrir um editor. Nas listas, edite os itens
                existentes e revise os dados antes de salvar. As alterações são
                persistidas no servidor.
              </p>
            </div>
            <p className="text-xs uppercase tracking-wider text-muted-foreground">
              Guias recomendados
            </p>
            {articles
              .filter(
                (a) =>
                  a.id === "inicio" ||
                  a.id === "otlp" ||
                  a.related.includes(current),
              )
              .map((a) => (
                <Link
                  key={a.id}
                  to="/$"
                  params={{ _splat: `ajuda/${a.id}` }}
                  onClick={() => setHelp(false)}
                  className="flex items-center justify-between rounded-lg border p-3 text-sm hover:bg-accent"
                >
                  {a.title}
                  <ArrowUpRight className="size-4" />
                </Link>
              ))}
            <Button variant="outline" asChild>
              <Link
                to="/$"
                params={{ _splat: "ajuda" }}
                onClick={() => setHelp(false)}
              >
                Abrir central de ajuda
              </Link>
            </Button>
          </div>
        </SheetContent>
      </Sheet>
    </>
  );
}

export function WorkspaceWelcome({
  snapshot,
  demo,
}: {
  snapshot: Snapshot;
  demo: boolean;
}) {
  const [dismissed, setDismissed] = useState(() => {
    try {
      return localStorage.getItem("observa-welcome-hidden") === "true";
    } catch {
      return false;
    }
  });
  return (
    <div className="mb-6 space-y-5">
      <section className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-4">
        <div>
          <h2 className="text-sm font-semibold">Acesso rápido</h2>
          <p className="mt-1 text-xs text-muted-foreground">
            Serviços, dashboards e configuração do ambiente.
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" asChild>
            <Link to="/$" params={{ _splat: "servicos" }}>
              <Activity className="size-4 menu-icon-cyan" />
              Serviços
            </Link>
          </Button>
          <Button size="sm" asChild>
            <Link to="/$" params={{ _splat: "dashboards/novo" }}>
              <Plus className="size-4" />
              Criar dashboard
            </Link>
          </Button>
        </div>
      </section>
      {!dismissed && (
        <section className="rounded-xl border border-border bg-surface p-5">
          <div className="mb-4 flex items-center justify-between">
            <div>
              <h2 className="font-semibold">Configuração inicial</h2>
              <p className="mt-1 text-xs text-muted-foreground">
                Conecte os agentes e configure o envio de telemetria.
              </p>
            </div>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Ocultar primeiros passos"
              onClick={() => {
                setDismissed(true);
                try {
                  localStorage.setItem("observa-welcome-hidden", "true");
                } catch {
                  /* storage unavailable */
                }
              }}
            >
              <X className="size-4" />
            </Button>
          </div>
          <div className="grid gap-3 md:grid-cols-3">
            {[
              {
                title: "Conectar infraestrutura",
                text: `${snapshot.hosts.length} hosts recebidos`,
                path: "agentes",
                icon: Server,
                done: !demo && snapshot.hosts.length > 0,
              },
              {
                title: "Instrumentar aplicações",
                text: "Envie traces, logs e métricas OTLP",
                path: "ajuda/otlp",
                icon: Activity,
                done:
                  !demo &&
                  snapshot.service_map.nodes.some((n) => n.kind === "service"),
              },
              {
                title: "Definir confiabilidade",
                text: "Crie seu primeiro objetivo de serviço",
                path: "slos",
                icon: ShieldCheck,
                done: false,
              },
            ].map((step) => (
              <Link
                key={step.path}
                to="/$"
                params={{ _splat: step.path }}
                className="flex gap-3 rounded-xl border bg-background/40 p-4 hover:border-primary/50"
              >
                <div className="grid size-9 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
                  {step.done ? (
                    <Check className="size-4" />
                  ) : (
                    <step.icon className="size-4" />
                  )}
                </div>
                <div>
                  <h3 className="text-sm font-medium">{step.title}</h3>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {step.text}
                  </p>
                </div>
              </Link>
            ))}
          </div>
        </section>
      )}
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {[
          {
            title: "Problemas APM",
            text: "Evidências, traces e linha do tempo",
            path: "problemas-apm",
            icon: Activity,
          },
          {
            title: "Mapa de serviços",
            text: "Dependências entre serviços",
            path: "service-mapping",
            icon: Workflow,
          },
          {
            title: "Preferências",
            text: "Tema, cores e densidade",
            path: "configuracoes",
            icon: Palette,
          },
          {
            title: "Ajuda",
            text: "Guias e solução de problemas",
            path: "ajuda",
            icon: BookOpen,
          },
        ].map((item) => (
          <Link
            key={item.path}
            to="/$"
            params={{ _splat: item.path }}
            className="workspace-shortcut group rounded-xl border border-border bg-surface p-4 transition-colors hover:border-primary/50"
          >
            <div className="mb-4 flex items-center justify-between">
              <item.icon className={cn("size-5", menuIconColor(item.path))} />
              <ArrowUpRight className="size-4 text-muted-foreground group-hover:text-primary" />
            </div>
            <h3 className="text-sm font-semibold">{item.title}</h3>
            <p className="mt-1 text-xs text-muted-foreground">{item.text}</p>
          </Link>
        ))}
      </div>
    </div>
  );
}

export function WorkspacePreferences() {
  const [value, setValue] = useState(loadAppearance);
  function update(patch: Partial<Appearance>) {
    const next = { ...value, ...patch };
    setValue(next);
    applyAppearance(next);
    try {
      localStorage.setItem("observa-appearance-v1", JSON.stringify(next));
      toast.success("Preferências salvas");
    } catch {
      toast.error("Não foi possível persistir neste navegador");
    }
  }
  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <WorkspaceHeading
        eyebrow="Interface"
        title="Preferências da interface"
        description="Ajuste a aparência para trabalhar com conforto. Suas preferências ficam neste navegador."
      />
      <div className="grid gap-6 lg:grid-cols-[1fr_300px]">
        <div className="space-y-5">
          <PreferenceCard
            title="Aparência"
            description="Selecione o tema da interface."
          >
            <div className="grid grid-cols-2 gap-3">
              {(["dark", "light"] as const).map((theme) => (
                <button
                  key={theme}
                  onClick={() => update({ theme })}
                  aria-pressed={value.theme === theme}
                  className={cn(
                    "rounded-xl border-2 p-4 text-left",
                    value.theme === theme
                      ? "border-primary bg-primary/5"
                      : "border-border",
                  )}
                >
                  <div
                    className={cn(
                      "mb-3 flex h-20 gap-2 rounded-lg border p-2",
                      theme === "dark" ? "bg-slate-950" : "bg-slate-100",
                    )}
                  >
                    <div className="w-1/4 rounded bg-violet-500/30" />
                    <div className="flex-1 space-y-2">
                      <div className="h-3 rounded bg-violet-500/40" />
                      <div className="h-10 rounded bg-slate-500/20" />
                    </div>
                  </div>
                  <span className="flex items-center gap-2 text-sm">
                    {theme === "dark" ? (
                      <Moon className="size-4" />
                    ) : (
                      <Sun className="size-4" />
                    )}
                    {theme === "dark" ? "Escuro" : "Claro"}
                    {value.theme === theme && (
                      <Check className="ml-auto size-4 text-primary" />
                    )}
                  </span>
                </button>
              ))}
            </div>
          </PreferenceCard>
          <PreferenceCard
            title="Cor de destaque"
            description="Personalize botões, navegação e destaques."
          >
            <div className="flex flex-wrap gap-3">
              {(["violet", "cyan", "green"] as const).map((accent) => (
                <button
                  key={accent}
                  onClick={() => update({ accent })}
                  aria-pressed={value.accent === accent}
                  className={cn(
                    "flex items-center gap-2 rounded-lg border px-4 py-3 text-sm",
                    value.accent === accent && "border-primary bg-primary/10",
                  )}
                >
                  <span
                    className={cn(
                      "size-4 rounded-full",
                      accent === "violet"
                        ? "bg-violet-400"
                        : accent === "cyan"
                          ? "bg-cyan-400"
                          : "bg-emerald-400",
                    )}
                  />
                  {{ violet: "Violeta", cyan: "Ciano", green: "Verde" }[accent]}
                </button>
              ))}
            </div>
          </PreferenceCard>
          <PreferenceCard
            title="Densidade"
            description="Controle o espaço das tabelas de investigação."
          >
            <div className="flex gap-3">
              {(["comfortable", "compact"] as const).map((density) => (
                <Button
                  key={density}
                  variant={value.density === density ? "default" : "outline"}
                  aria-pressed={value.density === density}
                  onClick={() => update({ density })}
                >
                  {density === "comfortable" ? "Confortável" : "Compacta"}
                </Button>
              ))}
            </div>
          </PreferenceCard>
          <Button
            variant="outline"
            onClick={() => {
              update(defaultAppearance);
              try {
                localStorage.removeItem("observa-welcome-hidden");
              } catch {
                /* optional storage */
              }
            }}
          >
            Restaurar preferências e primeiros passos
          </Button>
        </div>
        <aside className="h-fit rounded-xl border border-border bg-surface p-5">
          <Palette className="mb-4 text-primary" />
          <h2 className="font-semibold">Preferências salvas</h2>
          <p className="mt-3 text-sm leading-6 text-muted-foreground">
            A aparência muda imediatamente. Use Ctrl + K para navegar e Criar
            para abrir os editores disponíveis.
          </p>
          <div className="mt-5 border-t pt-5">
            <Button
              variant="outline"
              onClick={() => downloadJSON("observa-preferencias.json", value)}
            >
              <Download className="size-4" />
              Exportar preferências
            </Button>
          </div>
        </aside>
      </div>
    </div>
  );
}
function PreferenceCard({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-xl border border-border bg-surface p-5">
      <h2 className="font-semibold">{title}</h2>
      <p className="mb-5 mt-1 text-sm text-muted-foreground">{description}</p>
      {children}
    </section>
  );
}
export function WorkspaceHeading({
  eyebrow,
  title,
  description,
}: {
  eyebrow: string;
  title: string;
  description: string;
}) {
  return (
    <header>
      <p className="mb-2 text-xs font-semibold uppercase tracking-[.15em] text-primary">
        {eyebrow}
      </p>
      <h1 className="text-3xl font-semibold tracking-tight">{title}</h1>
      <p className="mt-3 max-w-2xl text-sm leading-6 text-muted-foreground">
        {description}
      </p>
    </header>
  );
}

const articles = [
  {
    id: "inicio",
    title: "Primeiros passos",
    category: "Primeiros passos",
    related: [""],
    body: "Conecte seu agente para descobrir hosts e processos. Depois, configure suas aplicações ou seu OpenTelemetry Collector para enviar telemetria. Em Serviços, escolha um serviço e o período; abra um trace para investigar falhas, atributos e logs correlacionados.",
    steps: [
      "Confira o inventário em Hosts e Agentes.",
      "Configure o envio OTLP pelo guia de conexão.",
      "Abra Serviços e investigue uma requisição.",
      "Crie dashboards, SLOs e regras conforme seus objetivos.",
    ],
  },
  {
    id: "otlp",
    title: "Conectar OpenTelemetry",
    category: "Integrações",
    related: ["servicos", "traces", "telemetria-logs", "metricas", "agentes"],
    body: "O servidor recebe OTLP por HTTP nos caminhos /v1/traces, /v1/logs e /v1/metrics. Configure o endereço que o Collector consegue alcançar e o token de agente no cabeçalho X-API-Key. O endereço localhost só funciona quando o exportador roda no mesmo host/rede do servidor.",
    steps: [
      "Use o token OBSERVA_AGENT_TOKEN configurado no servidor; não use o token administrativo no Collector.",
      "Para Collector em outro container, use uma rede compartilhada ou um endereço do host acessível. localhost nesse container aponta para ele mesmo.",
      "Para acesso remoto, configure publicação de rede e HTTPS apropriados.",
      "Envie sinais reais. Sem instrumentação, o inventário de processos não produz traces.",
    ],
  },
  {
    id: "dashboards",
    title: "Criar e editar dashboards",
    category: "Visualização",
    related: ["dashboards", "editor-paineis"],
    body: "Abra Criar → Criar dashboard. Informe um título, adicione painéis e selecione o serviço e a métrica. Você pode alternar entre indicador e série temporal, editar o dashboard existente, remover painéis ou exportar a definição JSON.",
    steps: [
      "Selecione um serviço que já recebeu spans.",
      "Escolha requisições, taxa de falhas, percentis ou requisições por segundo.",
      "Salve e aplique o período desejado.",
      "Use Editar para alterar a composição. Excluir pede confirmação.",
    ],
  },
  {
    id: "confiabilidade",
    title: "SLOs, regras e problemas",
    category: "Operação",
    related: ["slos", "correlacao", "problemas-apm"],
    body: "SLOs calculam disponibilidade ou latência a partir das requisições recebidas. Regras comparam taxa de falhas ou P95 a limites definidos por você. Problemas guardam evidências e uma linha do tempo; sem amostra suficiente, a ausência de dados não significa recuperação.",
    steps: [
      "Crie o objetivo e escolha serviço, tipo e meta.",
      "Crie uma regra com janela, limite e amostra mínima.",
      "Investigue as evidências e os traces de exemplo no problema.",
      "Resultados refletem a amostra retida, limitada a 24 horas nesta versão.",
    ],
  },
  {
    id: "diagnostico",
    title: "Resolver problemas de conexão",
    category: "Diagnóstico",
    related: ["configuracoes", "gateways", "data-sources"],
    body: "Se a conexão falhar, verifique primeiro o servidor, a publicação de portas e a autenticação. Não compartilhe tokens em capturas de tela ou chamados.",
    steps: [
      "401: confira o token administrativo usado na interface, ou o token de agente usado na ingestão.",
      "Sem dados: confira o período, o envio do Collector e os logs do servidor/agente.",
      "Porta ocupada: confira docker ps e ajuste OBSERVA_PORT no .env; recrie server e agent.",
      "Mudanças não aparecem: execute git pull e docker compose up -d --build server agent; recarregue o navegador.",
    ],
  },
];
const collectorConfig = `exporters:\n  otlphttp/observa:\n    endpoint: http://ENDERECO-DO-SERVIDOR:8080\n    headers:\n      X-API-Key: \${env:OBSERVA_AGENT_TOKEN}\n    compression: gzip\n\nservice:\n  pipelines:\n    traces:\n      receivers: [otlp]\n      exporters: [otlphttp/observa]\n    logs:\n      receivers: [otlp]\n      exporters: [otlphttp/observa]\n    metrics:\n      receivers: [otlp]\n      exporters: [otlphttp/observa]\n# Trecho para mesclar com a configuração existente do Collector.\n# O receiver otlp deve estar definido em receivers.`;
export function HelpCenter({
  path = "ajuda",
  snapshot,
  demo,
}: {
  path?: string;
  snapshot: Snapshot;
  demo: boolean;
}) {
  const [search, setSearch] = useState("");
  const selected = articles.find((a) => path === `ajuda/${a.id}`);
  async function copy() {
    try {
      await navigator.clipboard.writeText(collectorConfig);
      toast.success("Configuração copiada");
    } catch {
      toast.error("Não foi possível copiar. Selecione o trecho manualmente.");
    }
  }
  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <WorkspaceHeading
        eyebrow="Central de ajuda"
        title={selected?.title || "Central de ajuda"}
        description={
          selected
            ? selected.category
            : "Configuração, operação e solução de problemas."
        }
      />
      {selected ? (
        <>
          <Button variant="outline" asChild>
            <Link to="/$" params={{ _splat: "ajuda" }}>
              Todos os guias
            </Link>
          </Button>
          <section className="rounded-2xl border border-border bg-surface p-6 lg:p-8">
            <p className="max-w-3xl text-sm leading-7 text-muted-foreground">
              {selected.body}
            </p>
            <ol className="mt-6 space-y-4">
              {selected.steps.map((step, i) => (
                <li key={step} className="flex items-start gap-3 text-sm">
                  <span className="grid size-7 shrink-0 place-items-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
                    {i + 1}
                  </span>
                  <span className="pt-1 leading-6">{step}</span>
                </li>
              ))}
            </ol>
            {selected.id === "otlp" && (
              <div className="mt-6 overflow-hidden rounded-xl border">
                <div className="flex items-center justify-between border-b bg-background/60 px-4 py-2">
                  <span className="text-xs font-semibold">
                    Collector · trecho de configuração
                  </span>
                  <Button size="sm" variant="ghost" onClick={copy}>
                    <Copy className="size-3.5" />
                    Copiar
                  </Button>
                </div>
                <pre className="overflow-x-auto bg-background/70 p-5 text-xs leading-6">
                  <code>{collectorConfig}</code>
                </pre>
              </div>
            )}
          </section>
        </>
      ) : (
        <>
          <div className="relative max-w-xl">
            <Search className="absolute left-4 top-3.5 size-4 text-muted-foreground" />
            <Input
              aria-label="Buscar ajuda"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Busque por conexão, dashboards, SLOs…"
              className="h-12 rounded-xl bg-surface pl-11"
            />
          </div>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {articles
              .filter((a) =>
                `${a.title} ${a.body}`
                  .toLocaleLowerCase()
                  .includes(search.toLocaleLowerCase()),
              )
              .map((a) => (
                <Link
                  key={a.id}
                  to="/$"
                  params={{ _splat: `ajuda/${a.id}` }}
                  className="group rounded-2xl border border-border bg-surface p-6 transition hover:border-primary/50"
                >
                  <BookOpen className="mb-5 size-6 text-primary" />
                  <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                    {a.category}
                  </p>
                  <h2 className="mt-2 flex items-center justify-between font-semibold">
                    {a.title}
                    <ArrowUpRight className="size-4" />
                  </h2>
                  <p className="mt-3 line-clamp-3 text-sm leading-6 text-muted-foreground">
                    {a.body}
                  </p>
                </Link>
              ))}
          </div>
        </>
      )}
      <section className="flex flex-col gap-4 rounded-xl border border-border bg-surface p-5 md:flex-row md:items-center md:justify-between">
        <div>
          <h2 className="font-semibold">Informações para diagnóstico</h2>
          <p className="mt-1 text-xs text-muted-foreground">
            Resumo do workspace, sem tokens, nomes de hosts ou conteúdo de
            telemetria.
          </p>
        </div>
        <Button
          variant="outline"
          onClick={() =>
            downloadJSON("observa-diagnostico.json", {
              generated_at: new Date().toISOString(),
              mode: demo ? "demo" : "connected",
              snapshot_at: snapshot.generated_at,
              hosts: snapshot.hosts.length,
              services: snapshot.service_map.nodes.filter(
                (n) => n.kind === "service",
              ).length,
              dashboards: snapshot.dashboards.length,
              monitors: snapshot.templates.length,
              interface_preferences: loadAppearance(),
            })
          }
        >
          <Download className="size-4" />
          Exportar diagnóstico
        </Button>
      </section>
    </div>
  );
}

export function WorkspaceAccess() {
  const [token, setToken] = useState("");
  const [show, setShow] = useState(false);
  function reconnect(event: React.FormEvent) {
    event.preventDefault();
    if (token.trim().length < 24) return;
    sessionStorage.setItem("observa-lite-api-token", token.trim());
    window.location.reload();
  }
  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <WorkspaceHeading
        eyebrow="Administração / Acesso"
        title="Acesso ao workspace"
        description="Gerencie a credencial usada por esta aba para acessar o servidor."
      />
      <section className="rounded-xl border bg-surface p-6">
        <h2 className="font-semibold">Trocar token da conexão</h2>
        <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
          Informe um token administrativo já configurado no servidor. Ele fica
          na sessão desta aba. Esta ação troca a credencial da interface; para
          alterar o token aceito pelo servidor, atualize OBSERVA_ADMIN_TOKEN no
          .env e recrie o container server.
        </p>
        <form onSubmit={reconnect} className="mt-5 space-y-4">
          <label className="block text-sm">
            Token administrativo
            <Input
              type={show ? "text" : "password"}
              autoComplete="off"
              required
              minLength={24}
              value={token}
              onChange={(e) => setToken(e.target.value)}
              className="mt-2"
              placeholder="Cole o token configurado no servidor"
            />
          </label>
          <label className="flex items-center gap-2 text-xs text-muted-foreground">
            <input
              type="checkbox"
              checked={show}
              onChange={(e) => setShow(e.target.checked)}
            />
            Mostrar token digitado
          </label>
          <Button disabled={token.trim().length < 24}>
            Salvar e reconectar
          </Button>
        </form>
      </section>
      <section className="flex flex-col gap-4 rounded-xl border bg-surface p-6 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="font-semibold">Encerrar acesso nesta aba</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Remove o token da sessão e volta à conexão.
          </p>
        </div>
        <Button
          variant="outline"
          onClick={() => {
            sessionStorage.removeItem("observa-lite-api-token");
            window.location.reload();
          }}
        >
          Desconectar
        </Button>
      </section>
      <p className="rounded-xl border p-4 text-xs leading-6 text-muted-foreground">
        Autenticação atual por token administrativo. Contas individuais, perfis
        de acesso, SSO e MFA ainda não são aplicados por esta versão do
        servidor.
      </p>
    </div>
  );
}
