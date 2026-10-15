import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Activity,
  ArrowRight,
  Check,
  Code2,
  Download,
  Gauge,
  Globe2,
  Monitor,
  Pause,
  Play,
  Plus,
  Settings2,
  ShieldCheck,
  Smartphone,
  Trash2,
  Video,
} from "lucide-react";
import { request } from "@/lib/api";
import {
  newRUMApplication,
  replayPages,
  vitalHealth,
  type ReplayChunk,
  type RUMAnalysis,
  type RUMApplication,
  type RUMSession,
} from "@/lib/rum";
import { downloadJSON } from "@/lib/workspace";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "./ui/dialog";
import { Replayer } from "rrweb";
import "rrweb/dist/style.css";
const selectClass =
  "h-9 rounded-sm border border-input bg-surface px-3 text-xs";
const tabs = [
  { path: "rum", name: "RUM", icon: Globe2 },
  { path: "experience-vitals", name: "Experience Vitals", icon: Gauge },
  { path: "session-replay", name: "Session Replay", icon: Video },
  { path: "apdex", name: "Apdex", icon: Activity },
];
const pct = (n: number) => `${Math.round(n * 100)}%`;
const date = (s: string) => new Date(s).toLocaleString("pt-BR");
export function RUMWorkspace({ path, demo }: { path: string; demo: boolean }) {
  const qc = useQueryClient();
  const [appID, setAppID] = useState("");
  const [minutes, setMinutes] = useState(60);
  const [draft, setDraft] = useState<RUMApplication | null>(null);
  const [installation, setInstallation] = useState<RUMApplication | null>(null);
  const [deleting, setDeleting] = useState<RUMApplication | null>(null);
  const [selected, setSelected] = useState<RUMSession | null>(null);
  const [search, setSearch] = useState("");
  const [device, setDevice] = useState("");
  const apps = useQuery({
    queryKey: ["rum-apps"],
    queryFn: () =>
      request<{ applications: RUMApplication[] }>("/api/v1/rum/apps"),
    enabled: !demo,
  });
  const list = apps.data?.applications || [];
  const effective = appID || list[0]?.id || "";
  const data = useQuery({
    queryKey: ["rum-analysis", effective, minutes, device],
    queryFn: () =>
      request<RUMAnalysis>(
        `/api/v1/rum/analysis?app_id=${encodeURIComponent(effective)}&minutes=${minutes}&device=${encodeURIComponent(device)}`,
      ),
    enabled: !demo && !!effective,
    refetchInterval: 15000,
  });
  const a = data.data;
  const error = (e: unknown) =>
    e instanceof Error ? e.message : "Não foi possível concluir a operação.";
  const save = useMutation({
    mutationFn: (value: RUMApplication) =>
      request("/api/v1/rum/apps", {
        method: "POST",
        body: JSON.stringify(value),
      }),
    onSuccess: (_, value) => {
      void qc.invalidateQueries({ queryKey: ["rum-apps"] });
      void qc.invalidateQueries({ queryKey: ["rum-analysis"] });
      void qc.invalidateQueries({ queryKey: ["rum-replay"] });
      setAppID(value.id);
      setInstallation(value);
      setDraft(null);
    },
  });
  const remove = useMutation({
    mutationFn: (id: string) =>
      request(`/api/v1/rum/apps/${encodeURIComponent(id)}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      setDeleting(null);
      setAppID("");
      void qc.invalidateQueries({ queryKey: ["rum-apps"] });
      void qc.invalidateQueries({ queryKey: ["rum-analysis"] });
    },
  });
  const current = list.find((x) => x.id === effective);
  const sessions = (a?.sessions || []).filter((s) =>
    `${s.id} ${s.user} ${s.browser} ${s.os}`
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="mb-2 text-xs text-primary">Experiência digital</p>
          <h1 className="text-2xl font-semibold">
            {tabs.find((t) => t.path === path)?.name || "RUM"}
          </h1>
          <p className="mt-2 max-w-3xl text-sm text-muted-foreground">
            Navegação, desempenho e sessões reais de frontends web. Sem dados
            recebidos, os indicadores permanecem vazios.
          </p>
        </div>
        <Button
          disabled={demo}
          onClick={() => {
            save.reset();
            setDraft(newRUMApplication());
          }}
        >
          <Plus />
          Adicionar frontend
        </Button>
      </header>
      <nav
        aria-label="Experiência digital"
        className="flex flex-wrap gap-1 border-b pb-3"
      >
        {tabs.map((t) => (
          <Link
            key={t.path}
            to="/$"
            params={{ _splat: t.path }}
            className={`flex items-center gap-2 border px-3 py-2 text-xs ${path === t.path ? "border-primary bg-primary text-primary-foreground" : "border-border bg-surface"}`}
          >
            <t.icon className="size-4" />
            {t.name}
          </Link>
        ))}
      </nav>
      {demo && (
        <p className="border border-warning p-4 text-sm">
          O RUM não simula sessões no modo de demonstração. Conecte um servidor
          para configurar e receber eventos.
        </p>
      )}
      {(apps.error || data.error) && (
        <p role="alert" className="border border-destructive p-4 text-sm">
          {error(apps.error || data.error)}
        </p>
      )}
      {!!list.length && (
        <section className="flex flex-wrap items-center gap-3 border bg-surface p-3">
          <label className="text-xs">
            Frontend{" "}
            <select
              className={selectClass}
              aria-label="Selecionar frontend"
              value={effective}
              onChange={(e) => {
                setAppID(e.target.value);
                setSelected(null);
              }}
            >
              {list.map((app) => (
                <option key={app.id} value={app.id}>
                  {app.name}
                  {!app.enabled ? " · Pausado" : ""}
                </option>
              ))}
            </select>
          </label>
          <select
            aria-label="Janela RUM"
            className={selectClass}
            value={minutes}
            onChange={(e) => setMinutes(Number(e.target.value))}
          >
            <option value={15}>Últimos 15 minutos</option>
            <option value={60}>Última hora</option>
            <option value={360}>Últimas 6 horas</option>
            <option value={1440}>Últimas 24 horas</option>
          </select>
          <select
            aria-label="Filtrar dispositivo"
            className={selectClass}
            value={device}
            onChange={(e) => setDevice(e.target.value)}
          >
            <option value="">Todos os dispositivos</option>
            {["Desktop", "Mobile", "Tablet"].map((d) => (
              <option key={d}>{d}</option>
            ))}
          </select>
          <div className="ml-auto flex gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={() => {
                if (current) {
                  save.reset();
                  setDraft(current);
                }
              }}
            >
              <Settings2 />
              Configurar
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => setInstallation(current || null)}
            >
              <Code2 />
              Instalação
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => downloadJSON("vetra-rum.json", a)}
              disabled={!a}
            >
              <Download />
              Exportar
            </Button>
          </div>
        </section>
      )}
      {!demo && apps.isPending && (
        <p className="text-sm text-muted-foreground">Carregando frontends…</p>
      )}
      {!demo && !apps.isPending && !list.length && (
        <section className="grid gap-6 border bg-surface p-8 md:grid-cols-2">
          <div>
            <Globe2 className="mb-5 size-8 text-primary" />
            <h2 className="text-lg font-semibold">
              Conecte o primeiro frontend
            </h2>
            <p className="mt-3 text-sm leading-6 text-muted-foreground">
              Cadastre a origem exata, escolha as regras de privacidade e
              instale o JavaScript. O proxy de injeção permite inserir a tag sem
              editar o código-fonte da aplicação.
            </p>
            <Button
              className="mt-5"
              onClick={() => setDraft(newRUMApplication())}
            >
              Configurar captura web
              <ArrowRight />
            </Button>
          </div>
          <div className="space-y-4 border-l pl-6">
            <div className="flex gap-3">
              <ShieldCheck className="text-primary" />
              <p className="text-sm">
                Consentimento antes da coleta, formulários sempre mascarados e
                replay desativado por padrão.
              </p>
            </div>
            <div className="flex gap-3">
              <Smartphone className="text-primary" />
              <p className="text-sm">
                Sites acessados por celular usam o SDK web. Aplicativos Android
                e iOS nativos precisam de um SDK próprio; a Vetra ainda não
                fornece essa instrumentação.
              </p>
            </div>
          </div>
        </section>
      )}
      {a && (
        <>
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            {[
              ["Sessões", a.sessions_total],
              ["Page views", a.pages],
              ["Erros técnicos", a.errors],
              [
                "Satisfação estimada (Apdex)",
                a.apdex == null ? "—" : `${(a.apdex * 100).toFixed(1)}%`,
              ],
            ].map(([label, value]) => (
              <section key={label} className="border bg-surface p-5">
                <p className="text-xs text-muted-foreground">{label}</p>
                <p className="mt-3 text-3xl font-semibold">{value}</p>
              </section>
            ))}
          </div>
          {(path === "experience-vitals" || path === "rum") && (
            <section className="grid gap-3 md:grid-cols-3">
              {["LCP", "INP", "CLS"].map((name) => {
                const value = a.vitals_p75[name];
                const health = vitalHealth(name, value);
                return (
                  <div
                    key={name}
                    data-health={health}
                    className="vital-card border bg-surface p-5"
                  >
                    <div className="flex items-center justify-between">
                      <h2 className="font-semibold">{name}</h2>
                      <span className="text-[10px] text-muted-foreground">
                        p75 · {device || "Todos os dispositivos"}
                      </span>
                    </div>
                    <p className="my-3 text-2xl font-semibold">
                      {value == null
                        ? "—"
                        : name === "CLS"
                          ? value.toFixed(3)
                          : `${Math.round(value)} ms`}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {health === "good"
                        ? "Bom"
                        : health === "poor"
                          ? "Ruim"
                          : health === "needs-improvement"
                            ? "Precisa melhorar"
                            : "Aguardando amostras compatíveis"}
                    </p>
                  </div>
                );
              })}
            </section>
          )}
          {path === "experience-vitals" && (
            <div className="border bg-surface p-5 text-sm leading-6">
              <h2 className="mb-2 font-semibold">Como interpretar</h2>
              <p>
                LCP mede quando o maior conteúdo aparece; INP mede a resposta às
                interações; CLS mede mudanças inesperadas de layout. Ausência de
                INP pode significar que não houve interação válida ou suporte no
                navegador.
              </p>
              <p className="mt-2 text-muted-foreground">
                TTFB:{" "}
                {a.vitals_p75["TTFB"] == null
                  ? "—"
                  : `${Math.round(a.vitals_p75["TTFB"]!)} ms`}{" "}
                · FCP:{" "}
                {a.vitals_p75["FCP"] == null
                  ? "—"
                  : `${Math.round(a.vitals_p75["FCP"]!)} ms`}
                . As métricas usam eventos recebidos, não testes sintéticos.
              </p>
            </div>
          )}
          {path === "apdex" && (
            <section className="border bg-surface p-5">
              <h2 className="font-semibold">
                Satisfação por carregamento de página
              </h2>
              <p className="mt-2 text-sm text-muted-foreground">
                T = {current?.apdex_t_ms || a.apdex_t_ms} ms. Satisfeito até T;
                tolerável até 4T; frustrado acima de 4T ou com erro JavaScript
                observado na página. Fórmula: (Satisfeitos + Toleráveis / 2) /
                Total.
              </p>
              <div className="mt-5 grid grid-cols-3 gap-3">
                {[
                  ["Satisfeitos", a.apdex_counts.satisfied],
                  ["Toleráveis", a.apdex_counts.tolerated],
                  ["Frustrados", a.apdex_counts.frustrated],
                ].map(([label, value]) => (
                  <div key={label} className="border p-4">
                    <p className="text-xs">{label}</p>
                    <p className="mt-2 text-2xl">{value}</p>
                  </div>
                ))}
              </div>
              <p className="mt-4 text-xs text-muted-foreground">
                O Apdex usa carregamentos iniciais; navegações SPA sem duração
                medida não entram na conta. Sem amostras, ele é indefinido.
                Configure T para a expectativa real da sua aplicação.
              </p>
            </section>
          )}
          {path !== "apdex" && path !== "experience-vitals" && (
            <section className="overflow-hidden border bg-surface">
              <div className="flex items-center justify-between gap-3 border-b p-4">
                <h2 className="text-sm font-semibold">
                  {path === "session-replay"
                    ? "Sessões para investigação"
                    : "Sessões observadas"}
                </h2>
                <Input
                  className="max-w-sm"
                  aria-label="Buscar sessões"
                  placeholder="Buscar sessão, pseudônimo ou navegador"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <div className="overflow-auto">
                <table className="w-full text-left text-xs">
                  <thead className="bg-surface-raised text-muted-foreground">
                    <tr>
                      {[
                        "Sessão",
                        "Dispositivo",
                        "Navegador / Sistema",
                        "Páginas",
                        "Erros",
                        "Satisfação (Apdex)",
                        "Último evento",
                        "Ações",
                      ].map((h) => (
                        <th key={h} className="px-4 py-3 font-medium">
                          {h}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {sessions.map((s) => (
                      <tr key={s.app_id + s.id} className="border-t">
                        <td className="p-4">
                          <p className="font-mono">{s.id.slice(0, 12)}</p>
                          <p className="mt-1 text-muted-foreground">
                            {s.user
                              ? `Pseudônimo ${s.user.slice(0, 12)}`
                              : "Sessão sem identificação"}
                          </p>
                        </td>
                        <td className="p-4">{s.device}</td>
                        <td className="p-4">
                          {s.browser} / {s.os}
                        </td>
                        <td className="p-4">{s.pages}</td>
                        <td className="p-4">{s.errors}</td>
                        <td className="p-4">
                          {s.apdex == null
                            ? "—"
                            : `${(s.apdex * 100).toFixed(1)}%`}
                        </td>
                        <td className="p-4">{date(s.last)}</td>
                        <td className="p-4">
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => setSelected(s)}
                          >
                            <Video />
                            Investigar
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {!sessions.length && (
                <p className="p-10 text-center text-sm text-muted-foreground">
                  Nenhuma sessão recebida nesta seleção. Confira instalação,
                  origem permitida, consentimento e filtros.
                </p>
              )}
            </section>
          )}
          {path === "rum" && (
            <section className="overflow-auto border bg-surface">
              <h2 className="border-b p-4 text-sm font-semibold">
                Eventos recentes
              </h2>
              <table className="w-full text-left text-xs">
                <thead>
                  <tr>
                    {[
                      "Horário",
                      "Tipo",
                      "Origem / Caminho",
                      "Tempo / Valor",
                      "Status",
                    ].map((h) => (
                      <th key={h} className="p-3">
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {a.events
                    .slice(-100)
                    .reverse()
                    .map((e) => (
                      <tr key={e.id} className="border-t">
                        <td className="p-3">{date(e.at)}</td>
                        <td className="p-3">
                          {e.kind === "vital" ? e.name : e.kind}
                        </td>
                        <td className="max-w-md truncate p-3">{e.url}</td>
                        <td className="p-3">
                          {e.kind === "vital"
                            ? e.value.toFixed(3)
                            : e.kind === "error" || e.kind === "navigation"
                              ? "—"
                              : `${Math.round(e.duration_ms)} ms`}
                        </td>
                        <td className="p-3">
                          {e.error ? "Erro" : e.status || "—"}
                        </td>
                      </tr>
                    ))}
                </tbody>
              </table>
            </section>
          )}
          <p className="text-xs text-muted-foreground">
            Retenção de até 24 horas, limitada a 15.000 eventos e 25 MiB de
            replay por servidor. Telemetria enviada pelo navegador; não comprova
            identidade civil ou localização. IP, mensagens de formulário, corpos
            e cabeçalhos HTTP não são coletados.
          </p>
        </>
      )}
      <Dialog
        open={!!draft}
        onOpenChange={(v) => {
          if (!v) setDraft(null);
        }}
      >
        <DialogContent className="max-h-[90vh] overflow-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {list.some((a) => a.id === draft?.id)
                ? "Configurar frontend"
                : "Adicionar frontend web"}
            </DialogTitle>
            <DialogDescription>
              Origens e regras de captura são públicas. Tokens administrativos
              nunca devem ser colocados no navegador.
            </DialogDescription>
          </DialogHeader>
          {draft && (
            <form
              className="space-y-5"
              onSubmit={(e) => {
                e.preventDefault();
                save.mutate({
                  ...draft,
                  origins: draft.origins.map((v) => v.trim()).filter(Boolean),
                  public_text_selectors: draft.public_text_selectors
                    .map((v) => v.trim())
                    .filter(Boolean),
                  block_selectors: draft.block_selectors
                    .map((v) => v.trim())
                    .filter(Boolean),
                  exclude_paths: draft.exclude_paths
                    .map((v) => v.trim())
                    .filter(Boolean),
                });
              }}
            >
              <div className="grid gap-3 sm:grid-cols-2">
                <Field label="Nome">
                  <Input
                    required
                    maxLength={128}
                    value={draft.name}
                    onChange={(e) =>
                      setDraft({ ...draft, name: e.target.value })
                    }
                  />
                </Field>
                <Field label="ID">
                  <Input
                    required
                    disabled={list.some((a) => a.id === draft.id)}
                    pattern="[a-zA-Z0-9_-]{1,96}"
                    value={draft.id}
                    onChange={(e) => setDraft({ ...draft, id: e.target.value })}
                  />
                </Field>
              </div>
              <Field label="Origens permitidas">
                <Input
                  required
                  value={draft.origins.join(", ")}
                  placeholder="https://app.exemplo.com, http://localhost:3000"
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      origins: e.target.value.split(",").map((v) => v.trim()),
                    })
                  }
                />
              </Field>
              <div className="grid gap-4 sm:grid-cols-2">
                <Field label="Amostragem RUM (%)">
                  <Input
                    type="number"
                    min={0}
                    max={100}
                    value={draft.sample_rate * 100}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        sample_rate: Number(e.target.value) / 100,
                      })
                    }
                  />
                </Field>
                <Field label="Apdex T (ms)">
                  <Input
                    type="number"
                    min={100}
                    max={60000}
                    required
                    value={draft.apdex_t_ms}
                    onChange={(e) =>
                      setDraft({ ...draft, apdex_t_ms: Number(e.target.value) })
                    }
                  />
                </Field>
              </div>
              <fieldset className="space-y-3 border p-4">
                <legend className="px-2 text-xs font-semibold">
                  Captura e privacidade
                </legend>
                {(
                  [
                    ["enabled", "Captura habilitada"],
                    ["require_consent", "Exigir consentimento antes da coleta"],
                    [
                      "consent_banner",
                      "Exibir aviso de consentimento da Vetra",
                    ],
                    ["replay_enabled", "Permitir Session Replay"],
                    [
                      "capture_path",
                      "Capturar caminho das URLs (sem query string ou fragmento)",
                    ],
                    [
                      "identify_users",
                      "Permitir identificação por pseudônimo via API",
                    ],
                  ] as const
                ).map(([key, label]) => (
                  <label key={key} className="flex items-center gap-3 text-xs">
                    <input
                      type="checkbox"
                      checked={draft[key]}
                      onChange={(e) =>
                        setDraft({ ...draft, [key]: e.target.checked })
                      }
                    />
                    {label}
                  </label>
                ))}
                {!draft.require_consent && (
                  <p className="text-xs text-warning">
                    A coleta começará ao carregar a página. Defina essa opção
                    conforme a política do site e o mecanismo de consentimento
                    adotado.
                  </p>
                )}
                {draft.replay_enabled && (
                  <>
                    <Field label="Amostragem de replay (%)">
                      <Input
                        type="number"
                        min={0}
                        max={100}
                        value={draft.replay_sample_rate * 100}
                        onChange={(e) =>
                          setDraft({
                            ...draft,
                            replay_sample_rate: Number(e.target.value) / 100,
                          })
                        }
                      />
                    </Field>
                    <Field label="Mascaramento de texto">
                      <select
                        className={selectClass}
                        value={draft.masking}
                        onChange={(e) =>
                          setDraft({
                            ...draft,
                            masking: e.target
                              .value as RUMApplication["masking"],
                          })
                        }
                      >
                        <option value="all">Mascarar todos os textos</option>
                        <option value="allowlist">
                          Permitir apenas seletores públicos revisados
                        </option>
                      </select>
                    </Field>
                    {draft.masking === "allowlist" && (
                      <Field label="Seletores CSS de texto público (um por linha)">
                        <textarea
                          className="min-h-20 w-full border bg-background p-2"
                          value={draft.public_text_selectors.join("\n")}
                          onChange={(e) =>
                            setDraft({
                              ...draft,
                              public_text_selectors: e.target.value.split("\n"),
                            })
                          }
                        />
                      </Field>
                    )}
                    <Field label="Bloquear elementos por seletor CSS (um por linha)">
                      <textarea
                        className="min-h-20 w-full border bg-background p-2"
                        value={draft.block_selectors.join("\n")}
                        placeholder=".dados-pessoais"
                        onChange={(e) =>
                          setDraft({
                            ...draft,
                            block_selectors: e.target.value.split("\n"),
                          })
                        }
                      />
                    </Field>
                    <p className="text-xs text-muted-foreground">
                      Inputs, imagens, canvas e iframes permanecem protegidos.
                      Replay reconstrói estrutura, estilos de layout e
                      interações; não é uma gravação de vídeo pixel a pixel.
                    </p>
                  </>
                )}
                <Field label="Excluir caminhos por prefixo (um por linha)">
                  <textarea
                    className="min-h-20 w-full border bg-background p-2"
                    value={draft.exclude_paths.join("\n")}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        exclude_paths: e.target.value.split("\n"),
                      })
                    }
                  />
                </Field>
              </fieldset>
              {save.error && (
                <p role="alert" className="text-sm text-destructive">
                  {error(save.error)}
                </p>
              )}
              <div className="flex justify-between gap-3">
                {list.some((a) => a.id === draft.id) && (
                  <Button
                    type="button"
                    variant="destructive"
                    onClick={() => {
                      setDeleting(draft);
                      setDraft(null);
                    }}
                  >
                    <Trash2 />
                    Excluir frontend
                  </Button>
                )}
                <Button
                  disabled={save.isPending}
                  type="submit"
                  className="ml-auto"
                >
                  <Check />
                  {save.isPending ? "Salvando…" : "Salvar frontend"}
                </Button>
              </div>
            </form>
          )}
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!installation}
        onOpenChange={(v) => {
          if (!v) setInstallation(null);
        }}
      >
        <DialogContent className="max-h-[90vh] overflow-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Instalar captura web</DialogTitle>
            <DialogDescription>
              Use um endereço HTTPS do coletor acessível pelos navegadores dos
              usuários.
            </DialogDescription>
          </DialogHeader>
          {installation && <Installation app={installation} />}
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!deleting}
        onOpenChange={(v) => {
          if (!v) setDeleting(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Excluir {deleting?.name}?</DialogTitle>
            <DialogDescription>
              A configuração, os eventos e os replays retidos deste frontend
              serão excluídos.
            </DialogDescription>
          </DialogHeader>
          {remove.error && <p role="alert">{error(remove.error)}</p>}
          <Button
            variant="destructive"
            disabled={remove.isPending}
            onClick={() => deleting && remove.mutate(deleting.id)}
          >
            Confirmar exclusão
          </Button>
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!selected}
        onOpenChange={(v) => {
          if (!v) setSelected(null);
        }}
      >
        <DialogContent className="max-h-[95vh] overflow-auto sm:max-w-5xl">
          <DialogHeader>
            <DialogTitle>Investigar sessão</DialogTitle>
            <DialogDescription>
              {selected?.id} · {selected?.browser} · {selected?.os}
            </DialogDescription>
          </DialogHeader>
          {selected && (
            <SessionInvestigation
              session={selected}
              onDeleted={() => setSelected(null)}
            />
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <label className="block space-y-2 text-xs">
      <span>{label}</span>
      {children}
    </label>
  );
}
function Installation({ app }: { app: RUMApplication }) {
  const [origin, setOrigin] = useState(location.origin);
  const sdk = `${origin.replace(/\/$/, "")}/rum/vetra.js?app=${encodeURIComponent(app.id)}`;
  return (
    <div className="space-y-4 text-sm">
      <Field label="Endereço público da Vetra">
        <Input value={origin} onChange={(e) => setOrigin(e.target.value)} />
      </Field>
      <h3 className="font-semibold">Tag no HTML</h3>
      <pre className="overflow-auto border bg-background p-3 text-xs">{`<script src="${sdk}" crossorigin="anonymous"></script>`}</pre>
      <h3 className="font-semibold">
        Injeção por proxy sem editar os arquivos da aplicação
      </h3>
      <pre className="overflow-auto whitespace-pre-wrap border bg-background p-3 text-xs">{`docker build -f deploy/Dockerfile.rum-injector -t vetra-rum-proxy .\ndocker run -d --name vetra-rum-proxy -p 8081:8080 vetra-rum-proxy \\\n  -upstream http://ENDERECO-DA-APLICACAO:PORTA \\\n  -script-url '${sdk}'`}</pre>
      <p className="text-xs text-muted-foreground">
        O acesso à aplicação precisa passar pelo proxy. Configure TLS no
        ingress, revise autenticação, cookies, redirects, cache e origens.
        Respostas HTML com CSP são preservadas e não recebem injeção até o
        operador revisar a política. Consulte docs/RUM-INSTALLATION.md.
      </p>
      <h3 className="font-semibold">Consentimento e identidade</h3>
      <pre className="overflow-auto border bg-background p-3 text-xs">{`// Integração opcional com seu gerenciador de consentimento\nwindow.Vetra.setConsent({ rum: true, replay: false });\n// Revogar a captura e descartar buffers locais\nwindow.Vetra.setConsent(false);\n// Opcional, após login; habilite pseudônimos na configuração\nawait window.Vetra.identify("ID-INTERNO-OPACO");\nwindow.Vetra.resetIdentity();`}</pre>
      <p className="text-xs text-muted-foreground">
        A identificação não lê automaticamente nomes, emails ou campos do login.
        Não use CPF, email ou credenciais como identificador. O replay requer
        consentimento próprio e amostragem aprovada.
      </p>
      <div className="flex gap-3 border p-4">
        <Smartphone className="shrink-0 text-primary" />
        <p className="text-xs leading-5">
          Android e iOS nativos exigem instrumentação no build e um SDK
          específico. Esta entrega cobre navegadores e a parte web de WebViews
          compatíveis; não inclui um SDK mobile nativo, crash reporting nativo
          ou replay nativo.
        </p>
      </div>
    </div>
  );
}
function SessionInvestigation({
  session,
  onDeleted,
}: {
  session: RUMSession;
  onDeleted: () => void;
}) {
  const qc = useQueryClient();
  const replay = useQuery({
    queryKey: ["rum-replay", session.app_id, session.id],
    queryFn: () =>
      request<{ chunks: ReplayChunk[] }>(
        `/api/v1/rum/apps/${session.app_id}/sessions/${session.id}/replay`,
      ),
  });
  const pages = useMemo(
    () => replayPages(replay.data?.chunks || []),
    [replay.data],
  );
  const [page, setPage] = useState("");
  const [confirm, setConfirm] = useState(false);
  const purge = useMutation({
    mutationFn: () =>
      request(`/api/v1/rum/apps/${session.app_id}/sessions/${session.id}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["rum-analysis"] });
      void qc.invalidateQueries({ queryKey: ["rum-replay"] });
      onDeleted();
    },
  });
  return (
    <div className="min-w-0 max-w-full space-y-4">
      <div className="flex flex-wrap gap-4 text-xs">
        <span>{session.pages} páginas</span>
        <span>{session.errors} erros</span>
        <span>
          {date(session.started)} → {date(session.last)}
        </span>
        <span>
          {session.user ? `Pseudônimo ${session.user}` : "Sem identificação"}
        </span>
      </div>
      {replay.isPending && <p>Carregando replay…</p>}
      {replay.error && <p role="alert">Não foi possível consultar o replay.</p>}
      {!replay.isPending && !replay.error && !pages.size && (
        <p className="border p-6 text-sm text-muted-foreground">
          Não há replay completo retido para esta sessão. A captura pode estar
          desativada, fora da amostragem, sem consentimento ou sem o snapshot
          inicial.
        </p>
      )}
      {!!pages.size && (
        <>
          <select
            aria-label="Página do replay"
            className={selectClass}
            value={page || [...pages.keys()][0]}
            onChange={(e) => setPage(e.target.value)}
          >
            {[...pages.keys()].map((id, i) => (
              <option key={id} value={id}>
                Página {i + 1} · {id.slice(0, 8)}
              </option>
            ))}
          </select>
          <ReplayPlayer
            key={page || [...pages.keys()][0]}
            events={pages.get(page || [...pages.keys()][0]!) || []}
          />
        </>
      )}
      <div className="flex flex-wrap items-center justify-between gap-3 border-t pt-4">
        <Button
          variant="outline"
          onClick={() => downloadJSON("vetra-session-replay.json", replay.data)}
          disabled={!pages.size}
        >
          <Download />
          Exportar replay mascarado
        </Button>
        {!confirm ? (
          <Button variant="destructive" onClick={() => setConfirm(true)}>
            <Trash2 />
            Excluir dados desta sessão
          </Button>
        ) : (
          <Button
            variant="destructive"
            disabled={purge.isPending}
            onClick={() => purge.mutate()}
          >
            Confirmar exclusão dos eventos e replay
          </Button>
        )}
      </div>
      {purge.error && <p role="alert">Não foi possível excluir a sessão.</p>}
    </div>
  );
}
function ReplayPlayer({ events }: { events: any[] }) {
  const root = useRef<HTMLDivElement>(null);
  const player = useRef<Replayer | null>(null);
  const [problem, setProblem] = useState("");
  const [duration, setDuration] = useState(0);
  const [seek, setSeek] = useState(0);
  const [playing, setPlaying] = useState(false);
  useEffect(() => {
    if (!root.current || events.length < 2) return;
    try {
      const p = new Replayer(events, {
        root: root.current,
        showWarning: false,
        skipInactive: true,
        UNSAFE_replayCanvas: false,
        triggerFocus: false,
      });
      player.current = p;
      p.iframe.setAttribute("sandbox", "allow-same-origin");
      setDuration(p.getMetaData().totalTime);
      p.pause(0);
      const timer = setInterval(
        () => setSeek(Math.min(p.getCurrentTime(), p.getMetaData().totalTime)),
        250,
      );
      return () => {
        clearInterval(timer);
        p.destroy();
        player.current = null;
      };
    } catch {
      setProblem("O replay está incompleto ou não pôde ser reconstruído.");
      return;
    }
  }, [events]);
  return (
    <section className="min-w-0 max-w-full space-y-3">
      <div className="flex min-w-0 max-w-full flex-wrap items-center gap-2">
        <Button
          size="sm"
          onClick={() => {
            if (playing) player.current?.pause();
            else player.current?.play(seek);
            setPlaying(!playing);
          }}
        >
          {playing ? <Pause /> : <Play />}
          {playing ? "Pausar" : "Reproduzir"}
        </Button>
        <input
          aria-label="Posição do replay"
          type="range"
          min={0}
          max={duration}
          value={seek}
          onChange={(e) => {
            const n = Number(e.target.value);
            setSeek(n);
            player.current?.pause(n);
            setPlaying(false);
          }}
          className="w-0 min-w-24 flex-1"
        />
        <span className="text-xs">
          {Math.round(seek / 1000)} / {Math.round(duration / 1000)} s
        </span>
        <select
          aria-label="Velocidade do replay"
          className={selectClass}
          defaultValue={1}
          onChange={(e) =>
            player.current?.setConfig({ speed: Number(e.target.value) })
          }
        >
          {[0.5, 1, 2, 4].map((n) => (
            <option key={n} value={n}>
              {n}×
            </option>
          ))}
        </select>
      </div>
      {problem && <p role="alert">{problem}</p>}
      <div
        ref={root}
        className="vetra-replay-root min-w-0 max-w-full max-h-[55vh] overflow-auto border bg-white"
      />
      <p className="text-[11px] text-muted-foreground">
        Reconstrução mascarada. Recursos externos, scripts, imagens, iframes e
        canvas não são carregados; diferenças visuais são esperadas.
      </p>
    </section>
  );
}
