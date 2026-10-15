import { useState, type FormEvent } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import {
  api,
  request,
  type Dashboard,
  type DashboardPanel,
  type APMMetrics,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
const metrics = {
  requests: "Requisições",
  error_rate: "Taxa de falhas (%)",
  p50_ms: "P50 (ms)",
  p95_ms: "P95 (ms)",
  p99_ms: "P99 (ms)",
  requests_per_second: "Requisições/s",
};
type Metric = keyof typeof metrics;
function configuration(
  panel: DashboardPanel,
): { service_id: string; metric: Metric } | null {
  try {
    const p = JSON.parse(panel.query ?? "");
    return typeof p.service_id === "string" && Object.hasOwn(metrics, p.metric)
      ? p
      : null;
  } catch {
    return null;
  }
}
const rangeFor = (minutes: number) => {
  const to = new Date();
  return {
    from: new Date(to.getTime() - minutes * 60000).toISOString(),
    to: to.toISOString(),
  };
};
const error = (e: unknown) =>
  e instanceof Error ? e.message : "Erro desconhecido";
export function OperationalDashboards({
  items,
  demo,
}: {
  items: Dashboard[];
  demo: boolean;
}) {
  const client = useQueryClient();
  const [minutes, setMinutes] = useState(60),
    [range, setRange] = useState(() => rangeFor(60));
  const [draft, setDraft] = useState<Dashboard | null>(null),
    [deleting, setDeleting] = useState<Dashboard | null>(null);
  const hasAPM = items.some((d) =>
    d.panels.some((p) => p.kind === "apm-stat" || p.kind === "apm-series"),
  );
  const services = useQuery({
    queryKey: ["dashboard-services", range],
    queryFn: () => api.apmServices(range.from, range.to),
    enabled: !demo && (hasAPM || draft !== null),
    retry: false,
  });
  const series = useQuery({
    queryKey: ["dashboard-timeseries", range],
    queryFn: () => api.apmTimeseries(range.from, range.to),
    enabled:
      !demo && items.some((d) => d.panels.some((p) => p.kind === "apm-series")),
    retry: false,
  });
  const save = useMutation({
    mutationFn: (d: Dashboard) => api.createDashboard(d),
    onSuccess: () => {
      setDraft(null);
      client.invalidateQueries({ queryKey: ["dashboards"] });
    },
  });
  const remove = useMutation({
    mutationFn: (id: string) =>
      request(`/api/v1/dashboards/${encodeURIComponent(id)}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      setDeleting(null);
      client.invalidateQueries({ queryKey: ["dashboards"] });
    },
  });
  function updatePanel(index: number, patch: Partial<DashboardPanel>) {
    if (draft)
      setDraft({
        ...draft,
        panels: draft.panels.map((p, i) =>
          i === index ? { ...p, ...patch } : p,
        ),
      });
  }
  function submit(e: FormEvent) {
    e.preventDefault();
    if (draft && !demo) save.mutate(draft);
  }
  function exportJSON(d: Dashboard) {
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(d, null, 2)], { type: "application/json" }),
    );
    const a = document.createElement("a");
    a.href = url;
    a.download = `dashboard-${d.id}.json`;
    a.click();
    URL.revokeObjectURL(url);
  }
  return (
    <div className="space-y-5">
      <header>
        <div className="text-xs uppercase text-primary">
          Observar / Dashboards
        </div>
        <h1 className="mt-1 text-2xl font-semibold">Dashboards operacionais</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Monte indicadores e séries temporais com requisições realmente
          recebidas pelo APM.
        </p>
      </header>
      {demo ? (
        <p role="status">
          Modo demonstração: consultas reais e alterações desativadas.
        </p>
      ) : (
        <div className="flex flex-wrap items-center gap-3">
          <label>
            Intervalo
            <select
              aria-label="Intervalo"
              className="ml-2 rounded border border-border bg-background p-2"
              value={minutes}
              onChange={(e) => setMinutes(Number(e.target.value))}
            >
              <option value={15}>15 minutos</option>
              <option value={60}>1 hora</option>
              <option value={360}>6 horas</option>
              <option value={1440}>24 horas</option>
            </select>
          </label>
          <Button onClick={() => setRange(rangeFor(minutes))}>Atualizar</Button>
          <Button
            onClick={() => {
              save.reset();
              setDraft({
                id: `dashboard-${Date.now()}`,
                title: "",
                panels: [],
              });
            }}
          >
            Novo dashboard
          </Button>
        </div>
      )}
      <p className="text-xs text-muted-foreground">
        Janela: {new Date(range.from).toLocaleString("pt-BR")} até{" "}
        {new Date(range.to).toLocaleString("pt-BR")}. Atualizar aplica o
        intervalo selecionado.
      </p>
      {!items.length && (
        <p className="rounded border border-dashed border-border p-8 text-center">
          Nenhum dashboard configurado.
        </p>
      )}
      {items.map((d) => (
        <section key={d.id} className="space-y-3">
          <div className="flex flex-wrap items-center gap-3">
            <h2 className="mr-auto text-xl font-semibold">{d.title}</h2>
            <Button variant="outline" onClick={() => exportJSON(d)}>
              Exportar JSON
            </Button>
            {!demo && (
              <>
                <Button
                  variant="outline"
                  onClick={() => {
                    save.reset();
                    setDraft({ ...d, panels: d.panels.map((p) => ({ ...p })) });
                  }}
                >
                  Editar
                </Button>
                <Button
                  variant="outline"
                  onClick={() => {
                    remove.reset();
                    setDeleting(d);
                  }}
                >
                  Excluir
                </Button>
              </>
            )}
          </div>
          <div className="grid gap-4 lg:grid-cols-2">
            {d.panels.map((p) => {
              const config = configuration(p);
              const live = p.kind === "apm-stat" || p.kind === "apm-series";
              const service = services.data?.services.find(
                (s) => s.id === config?.service_id,
              );
              const buckets =
                series.data?.timeseries.find(
                  (s) => s.service_id === config?.service_id,
                )?.buckets ?? [];
              const failure =
                p.kind === "apm-series" ? series.error : services.error;
              return (
                <article
                  key={p.id}
                  className="min-w-0 rounded-lg border border-border bg-surface p-4"
                >
                  <h3 className="font-semibold">{p.title}</h3>
                  {live ? (
                    demo ? (
                      <p className="mt-4">—</p>
                    ) : !config ? (
                      <p role="alert">Configuração APM inválida.</p>
                    ) : failure ? (
                      <div role="alert">
                        Falha ao consultar: {error(failure)}
                        <Button
                          variant="outline"
                          onClick={() =>
                            p.kind === "apm-series"
                              ? series.refetch()
                              : services.refetch()
                          }
                        >
                          Tentar novamente
                        </Button>
                      </div>
                    ) : (
                        p.kind === "apm-series"
                          ? series.isFetching
                          : services.isFetching
                      ) ? (
                      <p role="status">Carregando…</p>
                    ) : (
                      <>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {config.service_id} · {metrics[config.metric]}
                        </p>
                        {p.kind === "apm-stat" ? (
                          <div className="py-5 text-3xl font-semibold">
                            {!service || service.requests === 0
                              ? "—"
                              : service[config.metric].toLocaleString("pt-BR", {
                                  maximumFractionDigits: 3,
                                })}
                          </div>
                        ) : !buckets.some((b) => b.requests > 0) ? (
                          <p className="py-5">
                            — Sem requisições nesta janela.
                          </p>
                        ) : (
                          <>
                            <div className="mt-4 h-48">
                              <ResponsiveContainer width="100%" height="100%">
                                <LineChart
                                  data={buckets.map((b) => ({
                                    ...b,
                                    value:
                                      b.requests === 0
                                        ? null
                                        : b[config.metric],
                                  }))}
                                >
                                  <XAxis
                                    dataKey="start_time"
                                    tickFormatter={(v) =>
                                      new Date(v).toLocaleTimeString("pt-BR", {
                                        hour: "2-digit",
                                        minute: "2-digit",
                                      })
                                    }
                                  />
                                  <YAxis />
                                  <Tooltip />
                                  <Line
                                    dataKey="value"
                                    name={metrics[config.metric]}
                                    type="linear"
                                    stroke="#22c55e"
                                    connectNulls={false}
                                    dot={false}
                                  />
                                </LineChart>
                              </ResponsiveContainer>
                            </div>
                            <details className="mt-2 text-xs">
                              <summary>Tabela de pontos</summary>
                              <table className="w-full">
                                <tbody>
                                  {buckets.map((b) => (
                                    <tr key={b.start_time}>
                                      <td>
                                        {new Date(b.start_time).toLocaleString(
                                          "pt-BR",
                                        )}
                                      </td>
                                      <td>
                                        {b.requests === 0
                                          ? "—"
                                          : b[config.metric]}
                                      </td>
                                    </tr>
                                  ))}
                                </tbody>
                              </table>
                            </details>
                          </>
                        )}
                      </>
                    )
                  ) : p.kind === "html" ? (
                    <>
                      <p className="my-2 text-xs text-muted-foreground">
                        Conteúdo HTML isolado; scripts desativados.
                      </p>
                      <iframe
                        title={p.title}
                        sandbox=""
                        srcDoc={p.html ?? ""}
                        className="h-48 w-full border-0"
                      />
                    </>
                  ) : p.kind === "stat" && !p.source_id ? (
                    <>
                      <p className="mt-2 text-xs text-muted-foreground">
                        Valor estático configurado
                      </p>
                      <div className="py-4 text-2xl">{p.query ?? "—"}</div>
                    </>
                  ) : (
                    <p className="mt-3 text-sm text-muted-foreground">
                      Painel legado preservado ({p.kind}). Fonte externa sem
                      visualização nesta tela.
                    </p>
                  )}
                </article>
              );
            })}
          </div>
        </section>
      ))}
      <Dialog
        open={!demo && draft !== null}
        onOpenChange={(open) => {
          if (!open && !save.isPending) setDraft(null);
        }}
      >
        <DialogContent className="max-h-[85vh] overflow-y-auto">
          <DialogTitle>
            {items.some((d) => d.id === draft?.id)
              ? "Editar dashboard"
              : "Novo dashboard"}
          </DialogTitle>
          <DialogDescription>
            Indicadores APM e séries temporais. Os painéis legados são
            preservados.
          </DialogDescription>
          {draft && (
            <form onSubmit={submit} className="space-y-4">
              <label>
                Título
                <Input
                  aria-label="Título do dashboard"
                  required
                  value={draft.title}
                  maxLength={256}
                  onChange={(e) =>
                    setDraft({ ...draft, title: e.target.value })
                  }
                />
              </label>
              {services.isError && (
                <p role="alert">
                  Falha ao carregar serviços: {error(services.error)}
                  <button type="button" onClick={() => services.refetch()}>
                    Tentar novamente
                  </button>
                </p>
              )}
              {draft.panels.map((p, i) => {
                const config = configuration(p);
                const live = p.kind === "apm-stat" || p.kind === "apm-series";
                return (
                  <fieldset
                    key={p.id}
                    className="space-y-2 rounded border border-border p-3"
                  >
                    <legend>Painel {i + 1}</legend>
                    <label>
                      Título
                      <Input
                        aria-label={`Título do painel ${i + 1}`}
                        required
                        value={p.title}
                        onChange={(e) =>
                          updatePanel(i, { title: e.target.value })
                        }
                      />
                    </label>
                    {live ? (
                      <>
                        <label>
                          Visualização
                          <select
                            aria-label={`Visualização ${i + 1}`}
                            value={p.kind}
                            onChange={(e) =>
                              updatePanel(i, { kind: e.target.value })
                            }
                          >
                            <option value="apm-stat">Indicador</option>
                            <option value="apm-series">Série temporal</option>
                          </select>
                        </label>
                        <label className="block">
                          Serviço
                          <select
                            aria-label={`Serviço ${i + 1}`}
                            className="ml-2 max-w-full bg-background"
                            required
                            value={config?.service_id ?? ""}
                            onChange={(e) =>
                              updatePanel(i, {
                                query: JSON.stringify({
                                  service_id: e.target.value,
                                  metric: config?.metric ?? "requests",
                                }),
                              })
                            }
                          >
                            <option value="">Selecione</option>
                            {config?.service_id &&
                              !services.data?.services.some(
                                (s) => s.id === config.service_id,
                              ) && (
                                <option value={config.service_id}>
                                  {config.service_id}
                                </option>
                              )}
                            {services.data?.services.map((s) => (
                              <option key={s.id} value={s.id}>
                                {s.name} ({s.namespace})
                              </option>
                            ))}
                          </select>
                        </label>
                        <label className="block">
                          Métrica
                          <select
                            aria-label={`Métrica ${i + 1}`}
                            className="ml-2 bg-background"
                            value={config?.metric ?? "requests"}
                            onChange={(e) =>
                              updatePanel(i, {
                                query: JSON.stringify({
                                  service_id: config?.service_id ?? "",
                                  metric: e.target.value,
                                }),
                              })
                            }
                          >
                            {Object.entries(metrics).map(([v, n]) => (
                              <option key={v} value={v}>
                                {n}
                              </option>
                            ))}
                          </select>
                        </label>
                      </>
                    ) : (
                      <p className="text-xs">
                        Configuração legada preservada: {p.kind}
                      </p>
                    )}
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() =>
                        setDraft({
                          ...draft,
                          panels: draft.panels.filter(
                            (_, index) => index !== i,
                          ),
                        })
                      }
                    >
                      Remover painel {i + 1}
                    </Button>
                  </fieldset>
                );
              })}
              <Button
                type="button"
                variant="outline"
                onClick={() =>
                  setDraft({
                    ...draft,
                    panels: [
                      ...draft.panels,
                      {
                        id: `panel-${Date.now()}`,
                        title: "Novo indicador",
                        kind: "apm-stat",
                        source_id: "observa-apm",
                        query: JSON.stringify({
                          service_id: "",
                          metric: "requests",
                        }),
                      },
                    ],
                  })
                }
              >
                Adicionar painel
              </Button>
              {save.isError && (
                <p role="alert">Falha ao salvar: {error(save.error)}</p>
              )}
              <div>
                <Button type="submit" disabled={save.isPending}>
                  {save.isPending ? "Salvando…" : "Salvar"}
                </Button>
                <Button
                  className="ml-2"
                  type="button"
                  variant="outline"
                  disabled={save.isPending}
                  onClick={() => setDraft(null)}
                >
                  Cancelar
                </Button>
              </div>
            </form>
          )}
        </DialogContent>
      </Dialog>
      <Dialog
        open={!demo && deleting !== null}
        onOpenChange={(open) => {
          if (!open && !remove.isPending) setDeleting(null);
        }}
      >
        <DialogContent>
          <DialogTitle>Excluir dashboard?</DialogTitle>
          <DialogDescription>
            Excluir {deleting?.title} e seus painéis?
          </DialogDescription>
          {remove.isError && (
            <p role="alert">Falha ao excluir: {error(remove.error)}</p>
          )}
          <Button
            disabled={remove.isPending}
            onClick={() => {
              if (deleting && !demo) remove.mutate(deleting.id);
            }}
          >
            Confirmar exclusão
          </Button>
          <Button
            variant="outline"
            disabled={remove.isPending}
            onClick={() => setDeleting(null)}
          >
            Cancelar
          </Button>
        </DialogContent>
      </Dialog>
    </div>
  );
}
