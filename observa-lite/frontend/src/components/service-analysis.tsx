import { useQuery } from "@tanstack/react-query";
import { request } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

type AnalysisGroup = {
  name: string;
  attributes: Record<string, string>;
  count: number;
  errors: number;
  p50_ms: number;
  p95_ms: number;
  p99_ms: number;
  total_duration_ms: number;
  mean_duration_ms: number;
  example_trace_ids: string[];
};
export type ServiceAnalysisResult = {
  service_id: string;
  from: string;
  to: string;
  count_unit: string;
  duration_meaning: string;
  observed_spans: number;
  failures: AnalysisGroup[];
  endpoints: AnalysisGroup[];
  database_calls: AnalysisGroup[];
  outbound_calls: AnalysisGroup[];
  messaging: AnalysisGroup[];
  instances: AnalysisGroup[];
  versions: AnalysisGroup[];
  truncated: boolean;
  group_limit: number;
};
const sections = [
  {
    key: "failures",
    label: "Falhas",
    empty: "Nenhuma falha ou exceção observada neste período.",
  },
  {
    key: "database_calls",
    label: "Banco de dados",
    empty:
      "Nenhuma chamada de banco identificada. A classificação depende de db.system nos spans.",
  },
  {
    key: "outbound_calls",
    label: "Chamadas externas",
    empty:
      "Nenhuma chamada externa identificada. São necessários spans client com endereço do destino.",
  },
  {
    key: "messaging",
    label: "Mensageria",
    empty:
      "Nenhuma operação de mensageria identificada. A classificação depende de messaging.system.",
  },
  {
    key: "instances",
    label: "Instâncias",
    empty:
      "Nenhuma instância identificada. Envie service.instance.id no recurso ou span.",
  },
  {
    key: "versions",
    label: "Versões",
    empty:
      "Nenhuma versão identificada. Envie service.version no recurso ou span.",
  },
] as const;
const format = (value: number) =>
  Number.isFinite(value)
    ? value.toLocaleString("pt-BR", { maximumFractionDigits: 2 })
    : "—";
const cell = "border-t border-border px-4 py-3 align-top";
export function ServiceAnalysis({
  serviceID,
  from,
  to,
  onTrace,
}: {
  serviceID: string;
  from: string;
  to: string;
  onTrace: (id: string) => void;
}) {
  const query = useQuery({
    queryKey: ["apm-analysis", serviceID, from, to],
    queryFn: () =>
      request<ServiceAnalysisResult>(
        "/api/v1/apm/analysis?" +
          new URLSearchParams({ service: serviceID, from, to }),
      ),
    enabled: !!serviceID,
    retry: false,
  });
  if (!serviceID) return null;
  if (query.isPending)
    return (
      <section
        aria-label="Investigação do serviço"
        role="status"
        className="rounded border border-border bg-surface p-6 text-sm text-muted-foreground"
      >
        Carregando investigação do serviço…
      </section>
    );
  if (query.isError)
    return (
      <section
        aria-label="Investigação do serviço"
        role="alert"
        className="rounded border border-danger/40 bg-surface p-4"
      >
        <p className="text-sm text-danger">
          Não foi possível consultar a investigação: {query.error.message}
        </p>
        <Button
          variant="outline"
          className="mt-3"
          onClick={() => void query.refetch()}
        >
          Tentar novamente
        </Button>
      </section>
    );
  const data = query.data;
  return (
    <section aria-label="Investigação do serviço" className="space-y-4">
      <header>
        <h2 className="font-semibold">Investigar serviço</h2>
        <p className="mt-1 text-xs text-muted-foreground">
          {format(data.observed_spans)} spans observados no período. Categorias
          podem se sobrepor. Duração acumulada soma spans e pode incluir
          chamadas simultâneas; não representa tempo de parede.
        </p>
        <p className="mt-1 text-xs text-muted-foreground">
          Instâncias e versões usam atributos recebidos; CPU, memória e saúde da
          infraestrutura não são inferidas.
        </p>
      </header>
      {data.truncated && (
        <p
          role="status"
          className="rounded border border-warning/40 p-3 text-sm text-warning"
        >
          Resultados limitados a {data.group_limit} grupos por categoria. Amplie
          a investigação pelos traces de exemplo.
        </p>
      )}
      <Tabs defaultValue="failures">
        <TabsList
          aria-label="Categorias de investigação"
          className="h-auto flex-wrap justify-start"
        >
          {sections.map((s) => (
            <TabsTrigger key={s.key} value={s.key}>
              {s.label} ({data[s.key]?.length ?? 0})
            </TabsTrigger>
          ))}
        </TabsList>
        {sections.map((s) => (
          <TabsContent key={s.key} value={s.key}>
            {data[s.key]?.length ? (
              <div className="overflow-x-auto rounded border border-border bg-surface">
                <table className="w-full text-left text-xs">
                  <thead>
                    <tr>
                      {[
                        "Grupo e evidências",
                        "Spans",
                        "Falhas",
                        "Média",
                        "P50",
                        "P95",
                        "P99",
                        "Duração acumulada",
                        "Traces de exemplo",
                      ].map((h) => (
                        <th
                          scope="col"
                          key={h}
                          className="whitespace-nowrap px-4 py-3 text-muted-foreground"
                        >
                          {h}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {data[s.key].map((g, index) => (
                      <tr key={JSON.stringify([g.name, g.attributes, index])}>
                        <td className={cell}>
                          <p className="mb-2 font-medium">
                            {g.name || "Não informado"}
                          </p>
                          <dl className="space-y-1">
                            {Object.entries(g.attributes ?? {}).map(
                              ([name, value]) => (
                                <div key={name}>
                                  <dt className="inline font-mono text-muted-foreground">
                                    {name}:{" "}
                                  </dt>
                                  <dd className="inline whitespace-pre-wrap break-all">
                                    {value || "Não informado"}
                                  </dd>
                                </div>
                              ),
                            )}
                          </dl>
                        </td>
                        <td className={cell}>{format(g.count)}</td>
                        <td className={cell}>{format(g.errors)}</td>
                        {[
                          g.mean_duration_ms,
                          g.p50_ms,
                          g.p95_ms,
                          g.p99_ms,
                          g.total_duration_ms,
                        ].map((value, i) => (
                          <td key={i} className={cell + " whitespace-nowrap"}>
                            {g.count ? format(value) + " ms" : "—"}
                          </td>
                        ))}
                        <td className={cell}>
                          <div className="space-y-2">
                            {g.example_trace_ids?.length ? (
                              g.example_trace_ids.map((id) => (
                                <button
                                  key={id}
                                  onClick={() => onTrace(id)}
                                  className="block break-all text-left font-mono text-primary hover:underline focus-visible:outline-2 focus-visible:outline-primary"
                                  aria-label={"Abrir trace " + id}
                                >
                                  {id}
                                </button>
                              ))
                            ) : (
                              <span className="text-muted-foreground">
                                Nenhum exemplo disponível
                              </span>
                            )}
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <p className="rounded border border-border bg-surface p-6 text-sm text-muted-foreground">
                {s.empty}
              </p>
            )}
          </TabsContent>
        ))}
      </Tabs>
    </section>
  );
}
