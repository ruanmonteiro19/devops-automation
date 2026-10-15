import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from "recharts";
import {
  Filter,
  RefreshCw,
  Download,
  ChartNoAxesCombined,
  History,
} from "lucide-react";
import { request } from "@/lib/api";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "./ui/dialog";
export type LatestItem = {
  id: string;
  host: string;
  host_id: string;
  group: string;
  name: string;
  key: string;
  value: string;
  unit: string;
  source: string;
  status: string;
  error?: string;
  timestamp: string;
  change?: string;
  numeric: boolean;
  history: boolean;
};
type Filters = {
  host: string;
  group: string;
  q: string;
  source: string;
  status: string;
  unsupported: boolean;
};
const empty: Filters = {
  host: "",
  group: "",
  q: "",
  source: "",
  status: "",
  unsupported: false,
};
const stamp = (value: string) =>
  value && new Date(value).getFullYear() > 1970
    ? new Date(value).toLocaleString("pt-BR")
    : "Nunca coletado";
function HistoryView({ item }: { item: LatestItem }) {
  const [minutes, setMinutes] = useState("60");
  const query = useQuery({
    queryKey: ["latest-history", item.id, minutes],
    queryFn: () => {
      const to = new Date();
      return request<{ items: LatestItem[] }>(
        `/api/v1/latest-data/${encodeURIComponent(item.id)}/history?${new URLSearchParams({ from: new Date(to.getTime() - Number(minutes) * 60000).toISOString(), to: to.toISOString() })}`,
      );
    },
  });
  const rows = query.data?.items ?? [];
  const points = rows
    .filter(
      (row) =>
        row.numeric &&
        Number.isFinite(Number(row.value)) &&
        Math.abs(Number(row.value)) <= Number.MAX_SAFE_INTEGER,
    )
    .map((row) => ({
      time: new Date(row.timestamp).getTime(),
      value: Number(row.value),
    }));
  return (
    <section className="space-y-4">
      <div className="flex flex-wrap justify-between gap-3">
        <p className="text-sm">
          {item.host} · {item.key}
        </p>
        <select
          aria-label="Período do histórico"
          className="border bg-background p-2 text-xs"
          value={minutes}
          onChange={(e) => setMinutes(e.target.value)}
        >
          <option value="60">Última hora</option>
          <option value="360">Últimas 6 horas</option>
          <option value="1440">Últimas 24 horas</option>
        </select>
      </div>
      {query.isLoading && <p role="status">Carregando histórico…</p>}
      {query.error && <p role="alert">{query.error.message}</p>}
      {points.length > 0 && (
        <div
          className="h-64 w-full border bg-background p-3"
          role="img"
          aria-label={`Gráfico de ${item.name}`}
        >
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={points}>
              <CartesianGrid stroke="#29382f" strokeDasharray="3 3" />
              <XAxis
                dataKey="time"
                type="number"
                domain={["dataMin", "dataMax"]}
                tickFormatter={(v) =>
                  new Date(Number(v)).toLocaleTimeString("pt-BR")
                }
                fontSize={10}
              />
              <YAxis domain={["auto", "auto"]} fontSize={10} />
              <Tooltip
                labelFormatter={(v) => stamp(new Date(Number(v)).toISOString())}
              />
              <Line
                type="linear"
                dataKey="value"
                stroke="#39c784"
                strokeWidth={2}
                dot={points.length < 30}
                isAnimationActive={false}
                connectNulls={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
      {!query.isLoading && !rows.length && (
        <p className="border p-4 text-sm text-muted-foreground">
          Não há amostras no período selecionado.
        </p>
      )}
      <p className="text-xs text-muted-foreground">
        Retenção nativa de até 24 horas, sujeita aos limites globais. Valores
        inteiros grandes são preservados na tabela e omitidos do gráfico para
        evitar perda de precisão.
      </p>
      <div className="max-h-72 overflow-auto border">
        <table className="w-full text-left text-xs">
          <thead className="sticky top-0 bg-surface-raised">
            <tr>
              <th className="p-2">Horário</th>
              <th className="p-2">Valor</th>
              <th className="p-2">Unidade</th>
            </tr>
          </thead>
          <tbody>
            {[...rows].reverse().map((row, i) => (
              <tr className="border-t" key={i}>
                <td className="p-2">{stamp(row.timestamp)}</td>
                <td className="p-2 font-mono break-all">{row.value}</td>
                <td className="p-2">{row.unit || "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
export function LatestDataPage({
  demo = false,
  graphs = false,
}: {
  demo?: boolean;
  graphs?: boolean;
}) {
  const [draft, setDraft] = useState<Filters>(() => ({
    ...empty,
    host: new URLSearchParams(location.search).get("host") || "",
  }));
  const [filters, setFilters] = useState(draft);
  const [chosen, setChosen] = useState<string[]>([]);
  const [history, setHistory] = useState<LatestItem[]>([]);
  const [page, setPage] = useState(0);
  const [sort, setSort] = useState<"host" | "timestamp" | "name">("host");
  const query = useQuery({
    queryKey: ["latest-data", filters],
    queryFn: () =>
      request<{ items: LatestItem[]; total: number }>(
        `/api/v1/latest-data?${new URLSearchParams({ ...filters, unsupported: String(filters.unsupported) })}`,
      ),
    enabled: !demo,
    refetchInterval: 30000,
  });
  const rows = [...(query.data?.items ?? [])]
    .filter((row) => !graphs || (row.history && row.numeric))
    .sort((a, b) =>
      sort === "timestamp"
        ? b.timestamp.localeCompare(a.timestamp)
        : (a[sort] + " " + a.key).localeCompare(b[sort] + " " + b.key),
    );
  const pages = Math.max(1, Math.ceil(rows.length / 50));
  const currentPage = Math.min(page, pages - 1);
  const shown = rows.slice(currentPage * 50, currentPage * 50 + 50);
  const selected = rows.filter((row) => chosen.includes(row.id));
  function csv() {
    const safe = (value: unknown) => {
      let text = String(value ?? "");
      if (/^[=+@\-\t\r]/.test(text)) text = "'" + text;
      return '"' + text.replaceAll('"', '""') + '"';
    };
    const fields = [
      "Host",
      "Grupo",
      "Item",
      "Chave",
      "Última coleta",
      "Valor",
      "Unidade",
      "Variação",
      "Estado",
    ];
    const text = [
      fields,
      ...(selected.length ? selected : rows).map((row) => [
        row.host,
        row.group,
        row.name,
        row.key,
        stamp(row.timestamp),
        row.value,
        row.unit,
        row.change,
        row.status,
      ]),
    ]
      .map((row) => row.map(safe).join(";"))
      .join("\r\n");
    const url = URL.createObjectURL(
      new Blob(["\ufeff" + text], { type: "text/csv;charset=utf-8" }),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = "vetra-dados-recentes.csv";
    link.click();
    URL.revokeObjectURL(url);
  }
  return (
    <div className="space-y-5">
      <header className="flex flex-wrap justify-between items-end gap-3">
        <div>
          <p className="mb-2 text-xs text-primary">Monitoramento</p>
          <h1 className="text-2xl font-semibold">
            {graphs ? "Gráficos" : "Dados recentes"}
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Últimos valores coletados, estado dos itens e acesso ao histórico.
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            disabled={demo || query.isFetching}
            onClick={() => void query.refetch()}
          >
            <RefreshCw className="size-4" />
            Atualizar
          </Button>
          <Button variant="outline" disabled={!rows.length} onClick={csv}>
            <Download className="size-4" />
            Exportar CSV
          </Button>
        </div>
      </header>
      <form
        className="border bg-surface p-4 space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          setFilters({ ...draft });
          setChosen([]);
          setPage(0);
        }}
      >
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
          {(
            [
              ["host", "Host ou equipamento"],
              ["group", "Grupo"],
              ["q", "Nome ou chave"],
            ] as const
          ).map(([key, label]) => (
            <label key={key} className="space-y-1 text-xs">
              {label}
              <Input
                value={draft[key]}
                onChange={(e) => setDraft({ ...draft, [key]: e.target.value })}
              />
            </label>
          ))}
          <label className="space-y-1 text-xs">
            Origem
            <select
              className="h-10 w-full border bg-background px-3"
              value={draft.source}
              onChange={(e) => setDraft({ ...draft, source: e.target.value })}
            >
              <option value="">Todas</option>
              {["SNMP", "OTLP", "Agente", "Monitor"].map((value) => (
                <option key={value}>{value}</option>
              ))}
            </select>
          </label>
          <label className="space-y-1 text-xs">
            Estado
            <select
              className="h-10 w-full border bg-background px-3"
              value={draft.status}
              onChange={(e) => setDraft({ ...draft, status: e.target.value })}
            >
              <option value="">Todos</option>
              {[
                "Disponível",
                "Falha de coleta",
                "Desatualizado",
                "Pausado",
                "Sem dados",
                "Não suportado",
              ].map((value) => (
                <option key={value}>{value}</option>
              ))}
            </select>
          </label>
        </div>
        <div className="flex flex-wrap gap-3 items-center">
          <label className="flex items-center gap-2 text-xs">
            <input
              type="checkbox"
              checked={draft.unsupported}
              onChange={(e) =>
                setDraft({ ...draft, unsupported: e.target.checked })
              }
            />
            Incluir itens não suportados
          </label>
          <Button type="submit" disabled={demo}>
            <Filter className="size-4" />
            Aplicar
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              setDraft({ ...empty });
              setFilters({ ...empty });
              setChosen([]);
              setPage(0);
            }}
          >
            Limpar filtros
          </Button>
        </div>
      </form>
      {demo && (
        <p className="border p-4 text-sm">
          Modo demonstração. Dados recentes requerem coleta real.
        </p>
      )}
      {query.error && (
        <p role="alert" className="border border-danger p-3 text-sm">
          {query.error.message}
        </p>
      )}
      {query.isLoading && <p role="status">Carregando itens…</p>}
      <div className="flex flex-wrap gap-3 items-center text-xs">
        <p>
          {rows.length} itens · {selected.length} selecionados
        </p>
        <label>
          Ordenar por{" "}
          <select
            className="border bg-background p-2"
            value={sort}
            onChange={(e) => setSort(e.target.value as typeof sort)}
          >
            <option value="host">Host</option>
            <option value="name">Item</option>
            <option value="timestamp">Última coleta</option>
          </select>
        </label>
        <Button
          variant="outline"
          size="sm"
          disabled={!selected.some((row) => row.history)}
          onClick={() =>
            setHistory(selected.filter((row) => row.history).slice(0, 8))
          }
        >
          <ChartNoAxesCombined className="size-4" />
          Ver gráficos selecionados (até 8)
        </Button>
      </div>
      <div className="overflow-x-auto border bg-surface">
        <table className="w-full min-w-[1000px] text-left text-xs">
          <thead className="bg-surface-raised">
            <tr>
              <th className="p-3">
                <input
                  aria-label="Selecionar itens da página"
                  type="checkbox"
                  checked={
                    shown.length > 0 &&
                    shown.every((row) => chosen.includes(row.id))
                  }
                  onChange={(e) =>
                    setChosen((ids) =>
                      e.target.checked
                        ? [...new Set([...ids, ...shown.map((row) => row.id)])]
                        : ids.filter(
                            (id) => !shown.some((row) => row.id === id),
                          ),
                    )
                  }
                />
              </th>
              {[
                "Host / Grupo",
                "Item / Chave",
                "Última coleta",
                "Último valor",
                "Variação",
                "Origem / Estado",
                "Ações",
              ].map((label) => (
                <th key={label} className="p-3">
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {shown.map((row) => (
              <tr key={row.id} className="border-t align-top">
                <td className="p-3">
                  <input
                    aria-label={`Selecionar ${row.host} ${row.key}`}
                    type="checkbox"
                    checked={chosen.includes(row.id)}
                    onChange={(e) =>
                      setChosen((ids) =>
                        e.target.checked
                          ? [...ids, row.id]
                          : ids.filter((id) => id !== row.id),
                      )
                    }
                  />
                </td>
                <td className="p-3 font-medium">
                  {row.host}
                  <span className="block mt-1 text-muted-foreground">
                    {row.group || "Sem grupo"}
                  </span>
                </td>
                <td className="p-3 max-w-80 break-words">
                  {row.name}
                  <code className="block mt-1 text-[10px] text-muted-foreground">
                    {row.key}
                  </code>
                </td>
                <td className="p-3 whitespace-nowrap">
                  {stamp(row.timestamp)}
                </td>
                <td className="p-3 max-w-72 break-all font-mono">
                  {row.value} {row.unit}
                </td>
                <td className="p-3 font-mono">{row.change ?? "—"}</td>
                <td className="p-3">
                  <span>{row.source}</span>
                  <span
                    className={`block mt-1 ${row.status === "Disponível" ? "text-primary" : "text-warning"}`}
                  >
                    {row.status}
                  </span>
                  {row.error && (
                    <details className="mt-2 max-w-64">
                      <summary className="cursor-pointer">Detalhes</summary>
                      <p className="mt-2 break-words">{row.error}</p>
                    </details>
                  )}
                </td>
                <td className="p-3">
                  {row.history ? (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => setHistory([row])}
                    >
                      <History className="size-3" />
                      Histórico
                    </Button>
                  ) : (
                    <span className="text-muted-foreground">Valor atual</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!shown.length && !query.isLoading && (
          <p className="p-8 text-sm text-muted-foreground">
            Nenhum item encontrado para os filtros atuais.
          </p>
        )}
      </div>
      <footer className="flex items-center justify-between text-xs">
        <span>
          Página {currentPage + 1} de {pages} · 50 itens por página
        </span>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={currentPage === 0}
            onClick={() => setPage(currentPage - 1)}
          >
            Anterior
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={currentPage >= pages - 1}
            onClick={() => setPage(currentPage + 1)}
          >
            Próxima
          </Button>
        </div>
      </footer>
      <Dialog
        open={history.length > 0}
        onOpenChange={(open) => {
          if (!open) setHistory([]);
        }}
      >
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-5xl">
          <DialogHeader>
            <DialogTitle>Histórico e gráficos</DialogTitle>
            <DialogDescription>
              Amostras recebidas pelo Vetra no período escolhido.
            </DialogDescription>
          </DialogHeader>
          {history.map((item) => (
            <HistoryView key={item.id} item={item} />
          ))}
        </DialogContent>
      </Dialog>
    </div>
  );
}
