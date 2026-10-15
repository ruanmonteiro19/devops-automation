import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { request } from "@/lib/api";
import type { TelemetryLog } from "@/components/telemetry-page";
import { Button } from "@/components/ui/button";

export function traceLogWindow(start: string, end: string, now = Date.now()) {
  const startMS = Date.parse(start),
    endMS = Date.parse(end);
  if (!Number.isFinite(startMS) || !Number.isFinite(endMS) || startMS > endMS)
    return null;
  const from = Math.max(now - 86400000 + 1000, startMS - 60000),
    to = Math.min(now, endMS + 60000);
  return from < to
    ? { from: new Date(from).toISOString(), to: new Date(to).toISOString() }
    : null;
}
export function TraceLogs({
  traceID,
  start,
  end,
  disabled = false,
}: {
  traceID: string;
  start: string;
  end: string;
  disabled?: boolean;
}) {
  const [range] = useState(() => traceLogWindow(start, end));
  const query = useQuery({
    queryKey: ["trace-logs", traceID, range],
    queryFn: () =>
      request<{ logs: TelemetryLog[]; truncated: boolean }>(
        "/api/v1/telemetry/logs?" +
          new URLSearchParams({
            trace_id: traceID,
            from: range!.from,
            to: range!.to,
            limit: "100",
          }),
      ),
    enabled: !disabled && !!range && !!traceID,
    retry: false,
  });
  return (
    <section
      aria-label="Logs correlacionados"
      className="space-y-3 rounded border border-border bg-surface p-4"
    >
      <h2 className="font-semibold">Logs correlacionados</h2>
      <p className="text-xs text-muted-foreground">
        Correlação exata por trace ID nos logs OTLP retidos. Janela de um minuto
        antes e depois do trace, limitada às últimas 24 horas. A ausência de
        logs pode indicar falta de instrumentação ou retenção.
      </p>
      {disabled ? (
        <p className="text-sm text-muted-foreground">
          Consulta indisponível no modo demonstração.
        </p>
      ) : !range ? (
        <p className="text-sm text-muted-foreground">
          Trace fora da janela de consulta de 24 horas ou período inválido.
        </p>
      ) : query.isError ? (
        <div role="alert" className="text-sm text-danger">
          Falha ao consultar logs: {query.error.message}
          <Button
            className="ml-3"
            variant="outline"
            onClick={() => void query.refetch()}
          >
            Tentar novamente
          </Button>
        </div>
      ) : query.isPending ? (
        <p role="status" className="text-sm text-muted-foreground">
          Carregando logs correlacionados…
        </p>
      ) : !query.data.logs?.length ? (
        <p className="text-sm text-muted-foreground">
          Nenhum log correlacionado encontrado no período.
        </p>
      ) : (
        <>
          {query.data.truncated && (
            <p className="text-xs text-warning">
              Resultado limitado a 100 logs.
            </p>
          )}
          <div className="max-h-96 overflow-auto">
            {query.data.logs.map((log, i) => (
              <article
                key={log.timestamp + ":" + log.span_id + ":" + i}
                className="border-t border-border py-3 text-xs"
              >
                <p className="text-muted-foreground">
                  <time>{new Date(log.timestamp).toLocaleString("pt-BR")}</time>{" "}
                  ·{" "}
                  {log.severity_text ||
                    (log.severity_number
                      ? "Severidade " + log.severity_number
                      : "Severidade não informada")}{" "}
                  · {log.service_name || "Serviço não informado"}
                </p>
                <p className="mt-1 break-all font-mono text-muted-foreground">
                  Span: {log.span_id || "não informado"}
                </p>
                <pre className="mt-2 whitespace-pre-wrap break-all font-mono">
                  {typeof log.body === "string"
                    ? log.body
                    : JSON.stringify(log.body, null, 2)}
                </pre>
              </article>
            ))}
          </div>
        </>
      )}
    </section>
  );
}
