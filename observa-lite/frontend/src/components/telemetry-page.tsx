import { useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { request } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
type Attr = Record<string, unknown>;
export type TelemetryLog = {
  timestamp: string;
  observed_timestamp?: string;
  service_name: string;
  severity_text?: string;
  severity_number: number;
  body: unknown;
  trace_id?: string;
  span_id?: string;
  attributes?: Attr;
  resource_attributes?: Attr;
};
export type TelemetryMetric = {
  timestamp: string;
  start_time?: string;
  name: string;
  description?: string;
  service_name: string;
  kind: string;
  unit?: string;
  temporality?: string;
  monotonic: boolean;
  data: Attr;
  attributes?: Attr;
  resource_attributes?: Attr;
};
type Result = {
  logs?: TelemetryLog[];
  metrics?: TelemetryMetric[];
  truncated: boolean;
};
const show = (v: unknown): string =>
  typeof v === "string" ? v : (JSON.stringify(v, null, 2) ?? "—");
const time = (v: string) =>
  new Date(v).toLocaleString("pt-BR", { timeZoneName: "short" });
function Attributes({
  row,
}: {
  row: { attributes?: Attr; resource_attributes?: Attr };
}) {
  return (
    <details className="mt-2">
      <summary className="cursor-pointer text-primary">
        Atributos e recurso
      </summary>
      <h4>Atributos</h4>
      <pre className="whitespace-pre-wrap break-all">
        {show(row.attributes ?? {})}
      </pre>
      <h4>Recurso</h4>
      <pre className="whitespace-pre-wrap break-all">
        {show(row.resource_attributes ?? {})}
      </pre>
    </details>
  );
}
export function TelemetryPage({
  path = "telemetria-logs",
  demo = false,
}: {
  path?: string;
  demo?: boolean;
}) {
  const logs = path === "telemetria-logs";
  const [minutes, setMinutes] = useState(60),
    [service, setService] = useState(
      () => new URLSearchParams(location.search).get("service") || "",
    ),
    [filter, setFilter] = useState(
      () =>
        new URLSearchParams(location.search).get(logs ? "trace_id" : "name") ||
        "",
    );
  const [submitted, setSubmitted] = useState<string | null>(() => {
    const query = new URLSearchParams(location.search);
    if (!query.get("service")) return null;
    const now = new Date();
    const p = new URLSearchParams({
      from: new Date(now.getTime() - 60 * 60000).toISOString(),
      to: now.toISOString(),
      limit: "200",
      service: query.get("service") || "",
    });
    if (query.get("name") && !logs) p.set("name", query.get("name") || "");
    return p.toString();
  });
  const query = useQuery({
    queryKey: ["telemetry", logs, submitted],
    queryFn: () =>
      request<Result>(
        `/api/v1/telemetry/${logs ? "logs" : "metrics"}?${submitted}`,
      ),
    enabled: !demo && submitted !== null,
    retry: false,
  });
  function submit(e: FormEvent) {
    e.preventDefault();
    if (demo) return;
    const to = new Date();
    const p = new URLSearchParams({
      from: new Date(to.getTime() - minutes * 60000).toISOString(),
      to: to.toISOString(),
      limit: "200",
    });
    if (service.trim()) p.set("service", service.trim());
    if (filter.trim()) p.set(logs ? "trace_id" : "name", filter.trim());
    setSubmitted(p.toString());
  }
  const rows = logs ? (query.data?.logs ?? []) : (query.data?.metrics ?? []);
  return (
    <div className="space-y-5">
      <header>
        <div className="text-xs uppercase text-primary">
          Explorar / OpenTelemetry
        </div>
        <h1 className="mt-1 text-2xl font-semibold">
          {logs ? "Logs OpenTelemetry" : "Métricas OpenTelemetry"}
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {logs
            ? "Registros recebidos por OTLP, com contexto de serviço e rastreamento."
            : "Pontos recebidos por OTLP. Valores, unidades e temporalidade preservados como enviados."}
        </p>
      </header>
      {demo ? (
        <p role="status" className="rounded-lg border border-border p-4">
          Modo demonstração: consultas reais desativadas. Nenhum dado de
          telemetria foi simulado.
        </p>
      ) : (
        <form
          onSubmit={submit}
          className="flex flex-wrap items-end gap-3 rounded-lg border border-border bg-surface p-4"
        >
          <label className="text-sm">
            Intervalo
            <select
              aria-label="Intervalo"
              className="mt-2 block h-9 rounded border border-border bg-background px-3"
              value={minutes}
              onChange={(e) => setMinutes(Number(e.target.value))}
            >
              <option value={15}>15 minutos</option>
              <option value={60}>1 hora</option>
              <option value={360}>6 horas</option>
              <option value={1440}>24 horas</option>
            </select>
          </label>
          <label className="text-sm">
            Serviço (exato)
            <Input
              aria-label="Serviço"
              className="mt-2"
              value={service}
              onChange={(e) => setService(e.target.value)}
              maxLength={256}
            />
          </label>
          <label className="text-sm">
            {logs ? "Trace ID (exato)" : "Nome da métrica (exato)"}
            <Input
              className="mt-2"
              aria-label={logs ? "Trace ID" : "Nome da métrica"}
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              maxLength={512}
            />
          </label>
          <Button type="submit" disabled={query.isFetching}>
            {query.isFetching
              ? "Consultando…"
              : submitted
                ? "Atualizar"
                : "Consultar"}
          </Button>
        </form>
      )}
      {!demo && submitted && (
        <p className="text-xs text-muted-foreground">
          Janela consultada: {time(new URLSearchParams(submitted).get("from")!)}{" "}
          até {time(new URLSearchParams(submitted).get("to")!)}. Atualizar
          recalcula a janela.
        </p>
      )}
      {!demo && query.isFetching && <p role="status">Carregando telemetria…</p>}
      {!demo && query.isError && (
        <div role="alert" className="rounded border border-danger p-4">
          Falha ao consultar:{" "}
          {query.error instanceof Error
            ? query.error.message
            : "erro desconhecido"}
          <Button className="ml-3" onClick={() => query.refetch()}>
            Tentar novamente
          </Button>
        </div>
      )}
      {!demo && query.isSuccess && !query.isFetching && (
        <section className="overflow-x-auto rounded-lg border border-border bg-surface">
          <div className="border-b border-border p-4 text-sm">
            {rows.length} {logs ? "registros" : "pontos"}
            {query.data.truncated && (
              <span className="ml-2 text-warning">
                Resultado limitado. Reduza a janela ou filtre o serviço.
              </span>
            )}
          </div>
          {!rows.length ? (
            <p className="p-8 text-center">
              Nenhum {logs ? "log" : "ponto de métrica"} encontrado nesta
              janela.
            </p>
          ) : (
            <table className="w-full text-left text-xs">
              <thead>
                <tr>
                  {(logs
                    ? [
                        "Horário",
                        "Serviço",
                        "Severidade",
                        "Mensagem e contexto",
                      ]
                    : [
                        "Horário",
                        "Serviço / Métrica",
                        "Tipo / Unidade",
                        "Temporalidade",
                        "Valor / Detalhes",
                      ]
                  ).map((h) => (
                    <th key={h} className="p-3">
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {logs
                  ? (rows as TelemetryLog[]).map((r, i) => (
                      <tr key={i} className="border-t border-border align-top">
                        <td className="p-3 whitespace-nowrap">
                          {time(r.timestamp)}
                        </td>
                        <td className="p-3">{r.service_name || "—"}</td>
                        <td className="p-3">
                          {r.severity_text || "Sem texto"} ({r.severity_number})
                        </td>
                        <td className="max-w-2xl p-3">
                          <pre className="whitespace-pre-wrap break-all">
                            {show(r.body)}
                          </pre>
                          <div className="mt-2 break-all">
                            Trace: {r.trace_id || "—"}
                            <br />
                            Span: {r.span_id || "—"}
                          </div>
                          <Attributes row={r} />
                          {r.observed_timestamp && (
                            <p>Observado: {time(r.observed_timestamp)}</p>
                          )}
                        </td>
                      </tr>
                    ))
                  : (rows as TelemetryMetric[]).map((r, i) => (
                      <tr key={i} className="border-t border-border align-top">
                        <td className="p-3 whitespace-nowrap">
                          {time(r.timestamp)}
                        </td>
                        <td className="p-3">
                          {r.service_name || "—"}
                          <div className="mt-1 font-mono">{r.name}</div>
                          {r.description && <p>{r.description}</p>}
                        </td>
                        <td className="p-3">
                          {r.kind}
                          <div>{r.unit || "Sem unidade"}</div>
                        </td>
                        <td className="p-3">
                          {r.temporality || "Não aplicável"}
                          {r.kind === "sum" && (
                            <div>Monotônica: {r.monotonic ? "Sim" : "Não"}</div>
                          )}
                        </td>
                        <td className="p-3">
                          <MetricValue metric={r} />
                          {r.start_time && <p>Início: {time(r.start_time)}</p>}
                          <Attributes row={r} />
                        </td>
                      </tr>
                    ))}
              </tbody>
            </table>
          )}
        </section>
      )}
      {!demo && !submitted && (
        <p className="rounded-lg border border-dashed border-border p-8 text-center text-sm text-muted-foreground">
          Selecione o intervalo e consulte os dados recebidos. A consulta é
          executada sob demanda.
        </p>
      )}
      {!logs && (
        <p className="text-xs text-muted-foreground">
          Somas cumulativas ou delta são exibidas sem conversão em taxas.
          Histogramas mostram os buckets recebidos; percentis não são estimados.
        </p>
      )}
    </div>
  );
}
function MetricValue({ metric: r }: { metric: TelemetryMetric }) {
  return r.kind.toLowerCase().includes("histogram") ? (
    <div>
      Contagem: {show(r.data["count"] ?? "—")}
      <br />
      Soma: {show(r.data["sum"] ?? "—")}
      <details className="mt-2">
        <summary className="cursor-pointer text-primary">
          Buckets do histograma
        </summary>
        <pre className="whitespace-pre-wrap break-all">{show(r.data)}</pre>
      </details>
    </div>
  ) : (
    <span className="font-mono">
      {show(r.data["asInt"] ?? r.data["asDouble"] ?? r.data["value"] ?? r.data)}
    </span>
  );
}
