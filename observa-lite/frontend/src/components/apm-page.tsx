import { TraceLogs } from "@/components/trace-logs";
import { ServiceAnalysis } from "@/components/service-analysis";
import { useState, type ReactNode } from "react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import { useQuery } from "@tanstack/react-query";
import { api, type APMService, type APMTrace, type APMSpan } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
const n = (v: number) =>
  Number.isFinite(v)
    ? v.toLocaleString("pt-BR", { maximumFractionDigits: 2 })
    : "—";
const cell = "border-t border-border px-4 py-3";
function Table({
  headers,
  children,
}: {
  headers: string[];
  children: ReactNode;
}) {
  return (
    <div className="overflow-x-auto rounded-lg border border-border bg-surface">
      <table className="w-full text-left text-sm">
        <thead>
          <tr>
            {headers.map((h) => (
              <th
                scope="col"
                key={h}
                className="px-4 py-3 text-xs text-muted-foreground"
              >
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  );
}
function Empty({ children }: { children: ReactNode }) {
  return (
    <p className="rounded-lg border border-border bg-surface p-8 text-center text-sm text-muted-foreground">
      {children}
    </p>
  );
}
function Attrs({ value }: { value: Record<string, unknown> | undefined }) {
  return value && Object.keys(value).length ? (
    <dl>
      {Object.entries(value).map(([k, v]) => (
        <div
          key={k}
          className="grid gap-2 border-b border-border py-2 text-xs sm:grid-cols-[220px_1fr]"
        >
          <dt className="break-all font-mono text-muted-foreground">{k}</dt>
          <dd className="whitespace-pre-wrap break-all">
            {typeof v === "string" ? v : JSON.stringify(v)}
          </dd>
        </div>
      ))}
    </dl>
  ) : (
    <p className="text-sm text-muted-foreground">Nenhum atributo recebido.</p>
  );
}
export function APMPage({
  path,
  demo,
  globalSearch,
  initialTrace = "",
}: {
  path: string;
  demo: boolean;
  globalSearch: string;
  initialTrace?: string;
}) {
  const serviceMode = ["servicos", "aplicacoes"].includes(path);
  const [minutes, setMinutes] = useState(60),
    [search, setSearch] = useState(""),
    [service, setService] = useState(""),
    [trace, setTrace] = useState(initialTrace),
    [failed, setFailed] = useState(path === "erros");
  const [range, setRange] = useState(() => makeRange(60));
  const services = useQuery({
    queryKey: ["apm-services", range],
    queryFn: () => api.apmServices(range.from, range.to),
    enabled: !demo,
    retry: false,
  });
  const traces = useQuery({
    queryKey: ["apm-traces", range, service, failed],
    queryFn: () => api.apmTraces(range.from, range.to, service, failed),
    enabled: !demo && (!serviceMode || !!service),
    retry: false,
  });
  const detail = useQuery({
    queryKey: ["apm-trace", trace, range],
    queryFn: () => api.apmTrace(trace),
    enabled: !demo && !!trace,
    retry: false,
  });
  const series = useQuery({
    queryKey: ["apm-timeseries", range],
    queryFn: () => api.apmTimeseries(range.from, range.to),
    enabled: !demo && !!service,
    retry: false,
  });
  const current = trace ? detail : serviceMode && !service ? services : traces;
  const needle = (globalSearch + " " + search).trim().toLowerCase();
  const items = (services.data?.services ?? []).filter((s) =>
    (s.name + " " + s.namespace).toLowerCase().includes(needle),
  );
  const selected = services.data?.services.find((s) => s.id === service);
  const results = (traces.data?.traces ?? []).filter((t) =>
    (
      t.trace_id +
      " " +
      (t.spans ?? []).map((s) => s.name + " " + s.service_name).join(" ")
    )
      .toLowerCase()
      .includes(needle),
  );
  function changeRange(m: number) {
    setMinutes(m);
    setRange(makeRange(m));
    setTrace("");
  }
  return (
    <div className="space-y-5">
      <header>
        <p className="text-xs uppercase text-primary">
          Aplicações / APM · OpenTelemetry
        </p>
        <h1 className="mt-1 text-2xl font-semibold">
          {trace
            ? "Detalhe do trace"
            : selected?.name ||
              (path === "erros"
                ? "Erros e exceções"
                : serviceMode
                  ? "Serviços APM"
                  : "Traces distribuídos")}
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Volume, falhas e latências calculados sobre spans retidos. Amostragem
          pode afetar estes indicadores.
        </p>
        {path === "aplicacoes" && (
          <p className="mt-2 text-xs text-muted-foreground">
            Visão de serviços instrumentados. Sessões de usuários e
            monitoramento de navegador ainda não disponíveis.
          </p>
        )}
      </header>
      <div className="flex flex-wrap items-end gap-3 rounded-lg border border-border bg-surface p-4">
        {(service || trace) && (
          <Button
            variant="outline"
            onClick={() => (trace ? setTrace("") : setService(""))}
          >
            Voltar
          </Button>
        )}
        <label className="grid gap-2 text-xs">
          Período
          <select
            className="rounded border border-input bg-background p-2"
            value={minutes}
            onChange={(e) => changeRange(Number(e.target.value))}
          >
            {[
              [15, "15 minutos"],
              [60, "1 hora"],
              [360, "6 horas"],
              [1440, "24 horas"],
            ].map(([v, l]) => (
              <option key={v} value={v}>
                {l}
              </option>
            ))}
          </select>
        </label>
        <label className="grid flex-1 gap-2 text-xs">
          Pesquisar
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Serviço, operação ou trace ID"
          />
        </label>
        {!serviceMode && (
          <label className="grid gap-2 text-xs">
            Serviço
            <select
              className="rounded border border-input bg-background p-2"
              value={service}
              onChange={(e) => {
                setService(e.target.value);
                setTrace("");
              }}
            >
              <option value="">Todos</option>
              {(services.data?.services ?? []).map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
          </label>
        )}
        {(!serviceMode || service) && (
          <label className="flex items-center gap-2 py-2 text-sm">
            <input
              type="checkbox"
              checked={failed}
              onChange={(e) => {
                setFailed(e.target.checked);
                setTrace("");
              }}
            />
            Somente falhas
          </label>
        )}
        <Button
          variant="outline"
          disabled={demo || current.isFetching}
          onClick={() => changeRange(minutes)}
        >
          Atualizar
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        {new Date(range.from).toLocaleString("pt-BR")} até{" "}
        {new Date(range.to).toLocaleString("pt-BR")} · horário local
      </p>
      {demo ? (
        <Empty>
          APM indisponível no modo demonstração. Envie spans reais para
          habilitar métricas e traces.
        </Empty>
      ) : current.isError ? (
        <div
          role="alert"
          className="rounded border border-danger p-4 text-sm text-danger"
        >
          Falha na consulta: {current.error.message}
          <Button
            variant="outline"
            className="ml-3"
            onClick={() => void current.refetch()}
          >
            Tentar novamente
          </Button>
        </div>
      ) : current.isPending ? (
        <div role="status">
          <Empty>Carregando telemetria…</Empty>
        </div>
      ) : trace && detail.data ? (
        <TraceDetail key={trace} trace={detail.data} />
      ) : serviceMode && !service ? (
        items.length ? (
          <Table
            headers={[
              "Serviço",
              "Namespace",
              "Instâncias",
              "Requisições",
              "Req/s",
              "Falhas",
              "P50",
              "P95",
              "P99",
            ]}
          >
            {items.map((s) => (
              <tr key={s.id}>
                <td className={cell}>
                  <button
                    className="text-primary hover:underline"
                    onClick={() => setService(s.id)}
                  >
                    {s.name}
                  </button>
                </td>
                <td className={cell}>{s.namespace || "—"}</td>
                <td className={cell}>{s.instances}</td>
                <td className={cell}>{n(s.requests)}</td>
                <td className={cell}>{n(s.requests_per_second)}</td>
                <td className={cell}>{n(s.error_rate)}%</td>
                <td className={cell}>
                  {s.requests ? n(s.p50_ms) + " ms" : "—"}
                </td>
                <td className={cell}>
                  {s.requests ? (s.requests ? n(s.p95_ms) + " ms" : "—") : "—"}
                </td>
                <td className={cell}>
                  {s.requests ? n(s.p99_ms) + " ms" : "—"}
                </td>
              </tr>
            ))}
          </Table>
        ) : (
          <Empty>
            Nenhum serviço instrumentado neste período. Descoberta de processos
            não gera métricas APM. Configure o exportador OpenTelemetry ou
            amplie o período.
          </Empty>
        )
      ) : (
        <>
          {selected && serviceMode && (
            <>
              <ServiceDetail service={selected} />
              <ServiceAnalysis
                key={selected.id}
                serviceID={selected.id}
                from={range.from}
                to={range.to}
                onTrace={setTrace}
              />
              {series.isPending ? (
                <Empty>Carregando séries temporais…</Empty>
              ) : series.isError ? (
                <p role="alert" className="text-sm text-danger">
                  Séries temporais indisponíveis: {series.error.message}
                </p>
              ) : (
                <ServiceCharts
                  buckets={
                    series.data?.timeseries.find(
                      (s) => s.service_id === selected.id,
                    )?.buckets ?? []
                  }
                />
              )}
            </>
          )}
          <h2 className="text-sm font-semibold">Traces observados</h2>
          {results.length ? (
            <Table
              headers={[
                "Trace / operação",
                "Serviços",
                "Início",
                "Duração",
                "Spans",
                "Falhas",
                "Completude",
              ]}
            >
              {results.map((t) => (
                <tr key={t.trace_id}>
                  <td className={cell}>
                    <button
                      className="text-left text-primary hover:underline"
                      onClick={() => setTrace(t.trace_id)}
                    >
                      {t.spans?.find((s) => !s.parent_span_id)?.name ||
                        t.spans?.[0]?.name ||
                        "Trace"}
                      <span className="block font-mono text-[10px]">
                        {t.trace_id}
                      </span>
                    </button>
                  </td>
                  <td className={cell}>
                    {[
                      ...new Set((t.spans ?? []).map((s) => s.service_name)),
                    ].join(", ")}
                  </td>
                  <td className={cell}>
                    {new Date(t.start_time).toLocaleString("pt-BR")}
                  </td>
                  <td className={cell}>{n(t.duration_ms)} ms</td>
                  <td className={cell}>{t.spans?.length ?? 0}</td>
                  <td className={cell}>
                    {(t.spans ?? []).filter((s) => failedSpan(s)).length}
                  </td>
                  <td className={cell}>
                    {t.incomplete ? "Parcial" : "Raiz presente"}
                  </td>
                </tr>
              ))}
            </Table>
          ) : (
            <Empty>Nenhum trace encontrado para estes filtros.</Empty>
          )}
        </>
      )}
    </div>
  );
}
function failedSpan(s: APMSpan) {
  const code = Number(
    s.attributes?.["http.response.status_code"] ??
      s.attributes?.["http.status_code"],
  );
  return s.status.toLowerCase() === "error" || code >= 500;
}
function makeRange(minutes: number) {
  const to = new Date();
  return {
    from: new Date(to.getTime() - minutes * 60000).toISOString(),
    to: to.toISOString(),
  };
}
function ServiceDetail({ service: s }: { service: APMService }) {
  return (
    <>
      <div className="grid gap-3 sm:grid-cols-3">
        {[
          ["Requisições", n(s.requests)],
          ["Falhas", n(s.error_rate) + "%"],
          ["P95", s.requests ? n(s.p95_ms) + " ms" : "—"],
        ].map(([title, value]) => (
          <div
            key={title}
            className="rounded border border-border bg-surface p-4"
          >
            <p className="text-xs text-muted-foreground">{title}</p>
            <p className="mt-2 text-xl font-semibold">{value}</p>
          </div>
        ))}
      </div>
      <p className="text-xs text-muted-foreground">
        {s.instances} instância(s) identificada(s) · {s.span_count} spans ·{" "}
        {s.namespace || "Sem namespace"}. A identificação individual depende de
        service.instance.id.
      </p>
      <h2 className="text-sm font-semibold">Operações / endpoints</h2>
      <Table
        headers={["Operação", "Requisições", "Falhas", "P50", "P95", "P99"]}
      >
        {(s.endpoints ?? []).map((e) => (
          <tr key={e.name}>
            <td className={cell}>{e.name}</td>
            <td className={cell}>{n(e.requests)}</td>
            <td className={cell}>{n(e.error_rate)}%</td>
            <td className={cell}>{e.requests ? n(e.p50_ms) + " ms" : "—"}</td>
            <td className={cell}>{e.requests ? n(e.p95_ms) + " ms" : "—"}</td>
            <td className={cell}>{e.requests ? n(e.p99_ms) + " ms" : "—"}</td>
          </tr>
        ))}
      </Table>
    </>
  );
}
function TraceDetail({ trace: t }: { trace: APMTrace }) {
  const [id, setID] = useState("");
  const spans = [...(t.spans ?? [])].sort(
    (a, b) => Date.parse(a.start_time) - Date.parse(b.start_time),
  );
  const selected = spans.find((s) => s.span_id === id);
  const total = Math.max(t.duration_ms, 1),
    origin = Date.parse(t.start_time);
  function depth(s: APMSpan) {
    let d = 0,
      p = s.parent_span_id;
    const seen = new Set([s.span_id]);
    while (p && d < 12) {
      if (seen.has(p)) break;
      seen.add(p);
      const parent = spans.find((v) => v.span_id === p);
      if (!parent) break;
      d++;
      p = parent.parent_span_id;
    }
    return d;
  }
  return (
    <>
      <div className="rounded border border-border bg-surface p-4">
        <p className="break-all font-mono text-xs">{t.trace_id}</p>
        <p className="mt-2 text-sm">
          {n(t.duration_ms)} ms · {spans.length} spans ·{" "}
          {t.incomplete
            ? "Trace parcial: relações parentais ausentes."
            : "Raiz presente; não garante que todos os spans tenham sido recebidos."}
        </p>
        {!!t.missing_parent_ids?.length && (
          <p className="mt-2 break-all text-xs text-warning">
            Parents ausentes: {t.missing_parent_ids.join(", ")}
          </p>
        )}
      </div>
      <section
        aria-label="Waterfall do trace"
        className="overflow-x-auto rounded border border-border bg-surface"
      >
        <div className="min-w-[650px]">
          <div className="grid grid-cols-[1fr_2fr] p-3 text-xs text-muted-foreground">
            <span>Serviço / operação</span>
            <span>0 até {n(t.duration_ms)} ms</span>
          </div>
          {spans.map((s) => {
            const start = Math.min(
              100,
              Math.max(0, ((Date.parse(s.start_time) - origin) / total) * 100),
            );
            const ms = Math.max(
              0,
              Date.parse(s.end_time) - Date.parse(s.start_time),
            );
            return (
              <button
                key={s.span_id}
                aria-pressed={id === s.span_id}
                onClick={() => setID(s.span_id)}
                className="grid w-full grid-cols-[1fr_2fr] items-center border-t border-border p-3 text-left text-xs hover:bg-muted focus-visible:outline-2 focus-visible:outline-primary"
              >
                <span
                  className="truncate"
                  style={{ paddingLeft: depth(s) * 12 }}
                >
                  <span
                    className={failedSpan(s) ? "text-danger" : "text-primary"}
                  >
                    {s.service_name}
                  </span>
                  <span className="block truncate">{s.name}</span>
                </span>
                <span className="relative block h-7 rounded bg-muted/40">
                  <span
                    className={
                      failedSpan(s)
                        ? "absolute top-1 h-5 rounded bg-danger"
                        : "absolute top-1 h-5 rounded bg-primary/60"
                    }
                    style={{
                      left: start + "%",
                      width:
                        Math.min(
                          100 - start,
                          Math.max(0.2, (ms / total) * 100),
                        ) + "%",
                    }}
                  />
                  <span className="absolute inset-0 grid place-items-center">
                    {n(ms)} ms
                  </span>
                </span>
              </button>
            );
          })}
        </div>
      </section>
      {selected ? (
        <section className="space-y-4 rounded border border-border bg-surface p-4">
          <h2 className="font-semibold">{selected.name}</h2>
          <p className="break-all text-xs text-muted-foreground">
            {selected.service_name} · {selected.kind} · {selected.status} ·
            Instância: {selected.service_instance_id || "não informada"} · Span:{" "}
            {selected.span_id} · Parent: {selected.parent_span_id || "raiz"}
          </p>
          {selected.status_message && (
            <p className="text-sm text-danger">{selected.status_message}</p>
          )}
          <h3 className="text-sm font-semibold">Instrumentação e contexto</h3>
          <Attrs
            value={{
              scope: selected.scope,
              scope_schema_url: selected.scope_schema_url,
              resource_schema_url: selected.resource_schema_url,
              trace_state: selected.trace_state,
              flags: selected.flags,
            }}
          />
          <h3 className="text-sm font-semibold">Vínculos com spans</h3>
          {selected.links?.length ? (
            selected.links.map((link, i) => (
              <div key={i} className="rounded border border-border p-3">
                <a
                  className="break-all text-sm underline"
                  href={`/traces/${link.trace_id}`}
                >
                  Trace: {link.trace_id}
                </a>
                <p className="break-all text-xs">Span: {link.span_id}</p>
                <Attrs
                  value={{
                    trace_state: link.trace_state,
                    flags: link.flags,
                    dropped_attributes_count: link.dropped_attributes_count,
                    ...link.attributes,
                  }}
                />
              </div>
            ))
          ) : (
            <p className="text-sm text-muted-foreground">
              Nenhum vínculo recebido.
            </p>
          )}
          <h3 className="text-sm font-semibold">Atributos do span</h3>
          <Attrs value={selected.attributes} />
          <h3 className="text-sm font-semibold">Recurso</h3>
          <Attrs value={selected.resource_attributes} />
          <h3 className="text-sm font-semibold">Eventos e exceções</h3>
          {selected.events?.length ? (
            selected.events.map((e, i) => (
              <div key={i} className="rounded border border-border p-3">
                <p className="mb-2 text-sm">
                  {e.name} · {new Date(e.time).toLocaleString("pt-BR")}
                </p>
                <Attrs value={e.attributes} />
              </div>
            ))
          ) : (
            <p className="text-sm text-muted-foreground">
              Nenhum evento ou exceção recebido.
            </p>
          )}
        </section>
      ) : (
        <Empty>
          Selecione um span para inspecionar atributos, instância e exceções.
        </Empty>
      )}
      <TraceLogs
        key={t.trace_id}
        traceID={t.trace_id}
        start={t.start_time}
        end={t.end_time}
      />
    </>
  );
}

function ServiceCharts({
  buckets,
}: {
  buckets: (import("@/lib/api").APMMetrics & {
    start_time: string;
    end_time: string;
  })[];
}) {
  return buckets.length ? (
    <div className="grid gap-4 xl:grid-cols-3">
      {[
        ["Taxa de requisições", "requests_per_second", "req/s"],
        ["Taxa de falhas", "error_rate", "%"],
        ["Latência P95", "p95_ms", "ms"],
      ].map(([title, key, unit]) => (
        <section
          key={key}
          aria-label={title}
          className="rounded border border-border bg-surface p-4"
        >
          <h2 className="mb-4 text-sm font-semibold">
            {title} ({unit})
          </h2>
          <ResponsiveContainer width="100%" height={180}>
            <LineChart data={buckets}>
              <XAxis
                dataKey="start_time"
                tickFormatter={(v) =>
                  new Date(String(v)).toLocaleTimeString("pt-BR", {
                    hour: "2-digit",
                    minute: "2-digit",
                  })
                }
                minTickGap={45}
                tick={{ fontSize: 10, fill: "#a3b6aa" }}
              />
              <YAxis width={45} tick={{ fontSize: 10, fill: "#a3b6aa" }} />
              <Tooltip
                labelFormatter={(v) =>
                  new Date(String(v)).toLocaleString("pt-BR")
                }
                contentStyle={{ background: "#17251c", borderColor: "#3b4e40" }}
              />
              <Line
                dataKey={key!}
                name={title!}
                stroke="#43d978"
                dot={false}
                strokeWidth={2}
                isAnimationActive={false}
              />
            </LineChart>
          </ResponsiveContainer>
          <p className="mt-2 text-[10px] text-muted-foreground">
            Intervalos sem spans mostram zero. Não representam disponibilidade.
          </p>
        </section>
      ))}
    </div>
  ) : (
    <Empty>Sem séries temporais para este serviço.</Empty>
  );
}
