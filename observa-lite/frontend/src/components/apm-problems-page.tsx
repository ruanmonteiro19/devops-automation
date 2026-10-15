import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, request } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

export type APMRule = {
  id: string;
  name: string;
  service_id: string;
  metric: string;
  threshold: number;
  min_requests: number;
  window_minutes: number;
  severity: string;
  enabled: boolean;
};
const blank: APMRule = {
  id: "",
  name: "",
  service_id: "",
  metric: "error_rate",
  threshold: 5,
  min_requests: 10,
  window_minutes: 5,
  severity: "warning",
  enabled: true,
};
export function APMProblemsPage({
  demo,
  path,
  initialCreate = false,
}: {
  demo: boolean;
  path: string;
  initialCreate?: boolean;
}) {
  const rulesMode = path === "correlacao";
  const client = useQueryClient();
  const [status, setStatus] = useState(""),
    [draft, setDraft] = useState<APMRule | null>(() =>
      initialCreate && !demo && rulesMode ? { ...blank } : null,
    ),
    [deleting, setDeleting] = useState<APMRule | null>(null),
    [selected, setSelected] = useState("");
  const [range] = useState(() => {
    const to = new Date();
    return {
      from: new Date(to.getTime() - 86400000).toISOString(),
      to: to.toISOString(),
    };
  });
  const rules = useQuery({
    queryKey: ["apm-rules"],
    queryFn: () => request<{ rules: APMRule[] }>("/api/v1/apm/rules"),
    enabled: !demo,
    refetchInterval: 30000,
    retry: false,
  });
  const problems = useQuery({
    queryKey: ["apm-problems", status],
    queryFn: () =>
      request<{ problems: APMProblem[] }>(
        "/api/v1/apm/problems" +
          (status ? "?" + new URLSearchParams({ status }) : ""),
      ),
    enabled: !demo && !rulesMode,
    refetchInterval: 30000,
    retry: false,
  });
  const services = useQuery({
    queryKey: ["apm-rule-services", range],
    queryFn: () => api.apmServices(range.from, range.to),
    enabled: !demo && rulesMode,
    retry: false,
  });
  function invalidate() {
    void client.invalidateQueries({ queryKey: ["apm-rules"] });
    void client.invalidateQueries({ queryKey: ["apm-problems"] });
  }
  const save = useMutation({
    mutationFn: (policy: APMRule) =>
      request("/api/v1/apm/rules", {
        method: "POST",
        body: JSON.stringify(policy),
      }),
    onSuccess: () => {
      setDraft(null);
      invalidate();
    },
  });
  const remove = useMutation({
    mutationFn: (id: string) =>
      request("/api/v1/apm/rules/" + encodeURIComponent(id), {
        method: "DELETE",
      }),
    onSuccess: () => {
      setDeleting(null);
      invalidate();
    },
  });
  const evaluate = useMutation({
    mutationFn: () => request("/api/v1/apm/rules/evaluate", { method: "POST" }),
    onSuccess: invalidate,
  });
  const current = rulesMode ? rules : problems;
  const mutationError = save.error || remove.error || evaluate.error;
  function submit(event: FormEvent) {
    event.preventDefault();
    if (!draft || demo) return;
    save.mutate({ ...draft, id: draft.id || crypto.randomUUID() });
  }
  return (
    <div className="space-y-5">
      <header>
        <p className="text-xs uppercase text-primary">Aplicações / APM</p>
        <h1 className="mt-1 text-2xl font-semibold">
          {rulesMode ? "Regras de problemas APM" : "Problemas APM"}
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Detecção por regras sobre requisições instrumentadas retidas. Não
          determina causa raiz nem estima usuários afetados. Ausência de dados
          não confirma recuperação.
        </p>
      </header>
      <div className="flex flex-wrap items-center gap-3">
        {rulesMode ? (
          <Button
            disabled={demo}
            onClick={() => {
              save.reset();
              setDraft({ ...blank });
            }}
          >
            Nova regra
          </Button>
        ) : (
          <label className="flex items-center gap-2 text-sm">
            Estado
            <select
              className="rounded border border-input bg-background p-2"
              value={status}
              onChange={(e) => {
                setStatus(e.target.value);
                setSelected("");
              }}
            >
              <option value="">Todos</option>
              <option value="active">Ativos</option>
              <option value="resolved">Resolvidos</option>
            </select>
          </label>
        )}
        <Button
          variant="outline"
          disabled={demo || current.isFetching}
          onClick={() => void current.refetch()}
        >
          Atualizar
        </Button>
        <Button
          variant="outline"
          disabled={demo || evaluate.isPending}
          onClick={() => evaluate.mutate()}
        >
          {evaluate.isPending ? "Avaliando…" : "Avaliar regras"}
        </Button>
        <p className="text-xs text-muted-foreground">
          Atualização a cada 30 segundos
        </p>
      </div>
      {demo ? (
        <Message>
          Problemas APM e alterações de regras indisponíveis no modo
          demonstração.
        </Message>
      ) : current.isError ? (
        <div role="alert" className="text-sm text-danger">
          Falha na consulta: {current.error.message}
          <Button
            className="ml-3"
            variant="outline"
            onClick={() => void current.refetch()}
          >
            Tentar novamente
          </Button>
        </div>
      ) : current.isPending ? (
        <div role="status">
          <Message>Carregando…</Message>
        </div>
      ) : rulesMode ? (
        <div className="space-y-3">
          {rules.data?.rules.length ? (
            rules.data.rules.map((r) => (
              <article
                key={r.id}
                className="flex flex-wrap items-center justify-between gap-3 rounded border border-border bg-surface p-4"
              >
                <div>
                  <h2 className="font-medium">{r.name}</h2>
                  <p className="mt-1 break-all text-xs text-muted-foreground">
                    {r.service_id} · {r.metric} ≥ {r.threshold}
                    {r.metric === "error_rate" ? "%" : " ms"} · Janela:{" "}
                    {r.window_minutes} min · Mínimo: {r.min_requests}{" "}
                    requisições · {r.severity} ·{" "}
                    {r.enabled ? "Habilitada" : "Desabilitada"}
                  </p>
                </div>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    onClick={() => {
                      save.reset();
                      setDraft({ ...r });
                    }}
                  >
                    Editar
                  </Button>
                  <Button
                    variant="outline"
                    onClick={() => {
                      remove.reset();
                      setDeleting(r);
                    }}
                  >
                    Excluir
                  </Button>
                </div>
              </article>
            ))
          ) : (
            <Message>Nenhuma regra configurada.</Message>
          )}
        </div>
      ) : (
        <div className="space-y-3">
          {problems.data?.problems.length ? (
            problems.data.problems.map((p) => (
              <ProblemCard
                key={p.id}
                problem={p}
                expanded={selected === p.id}
                onOpen={() => setSelected(selected === p.id ? "" : p.id)}
              />
            ))
          ) : (
            <Message>
              Nenhum problema encontrado para este filtro. Isso não confirma a
              saúde dos serviços.
            </Message>
          )}
        </div>
      )}
      {mutationError && (
        <p
          role="alert"
          className="rounded border border-danger/40 p-3 text-sm text-danger"
        >
          Operação não concluída: {mutationError.message}
        </p>
      )}
      <Dialog
        open={!!draft}
        onOpenChange={(open) => {
          if (!open && !save.isPending) setDraft(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {draft?.id ? "Editar regra" : "Nova regra"}
            </DialogTitle>
            <DialogDescription>
              O limite dispara quando o valor observado atinge ou excede o
              limiar e há amostra mínima.
            </DialogDescription>
          </DialogHeader>
          {draft && (
            <form onSubmit={submit} className="space-y-4">
              <label className="grid gap-2 text-sm">
                Nome
                <Input
                  required
                  maxLength={120}
                  value={draft.name}
                  onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                />
              </label>
              <label className="grid gap-2 text-sm">
                Serviço
                <select
                  required
                  className="rounded border border-input bg-background p-2"
                  value={draft.service_id}
                  onChange={(e) =>
                    setDraft({ ...draft, service_id: e.target.value })
                  }
                >
                  <option value="">Selecionar serviço</option>
                  {draft.service_id &&
                    !services.data?.services.some(
                      (s) => s.id === draft.service_id,
                    ) && (
                      <option value={draft.service_id}>
                        {draft.service_id} (sem tráfego recente)
                      </option>
                    )}
                  {services.data?.services.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.namespace ? s.namespace + " / " : ""}
                      {s.name}
                    </option>
                  ))}
                </select>
              </label>
              {services.isError && (
                <p role="alert" className="text-xs text-danger">
                  Não foi possível listar serviços: {services.error.message}
                </p>
              )}
              <div className="grid grid-cols-2 gap-3">
                <label className="grid gap-2 text-sm">
                  Métrica
                  <select
                    className="rounded border border-input bg-background p-2"
                    value={draft.metric}
                    onChange={(e) =>
                      setDraft({ ...draft, metric: e.target.value })
                    }
                  >
                    <option value="error_rate">Taxa de falhas (%)</option>
                    <option value="p95_ms">Latência P95 (ms)</option>
                  </select>
                </label>
                <label className="grid gap-2 text-sm">
                  Limiar
                  <Input
                    type="number"
                    required
                    min={0}
                    max={draft.metric === "error_rate" ? 100 : undefined}
                    step="any"
                    value={draft.threshold}
                    onChange={(e) =>
                      setDraft({ ...draft, threshold: Number(e.target.value) })
                    }
                  />
                </label>
                <label className="grid gap-2 text-sm">
                  Amostra mínima
                  <Input
                    type="number"
                    required
                    min={1}
                    max={20000}
                    step={1}
                    value={draft.min_requests}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        min_requests: Number(e.target.value),
                      })
                    }
                  />
                </label>
                <label className="grid gap-2 text-sm">
                  Janela (minutos)
                  <Input
                    type="number"
                    required
                    min={1}
                    max={60}
                    step={1}
                    value={draft.window_minutes}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        window_minutes: Number(e.target.value),
                      })
                    }
                  />
                </label>
              </div>
              <label className="grid gap-2 text-sm">
                Severidade
                <select
                  className="rounded border border-input bg-background p-2"
                  value={draft.severity}
                  onChange={(e) =>
                    setDraft({ ...draft, severity: e.target.value })
                  }
                >
                  <option value="warning">Aviso</option>
                  <option value="critical">Crítica</option>
                </select>
              </label>
              <label className="flex gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={draft.enabled}
                  onChange={(e) =>
                    setDraft({ ...draft, enabled: e.target.checked })
                  }
                />
                Habilitada
              </label>
              <Button
                type="submit"
                disabled={demo || save.isPending || !draft.service_id}
              >
                {save.isPending ? "Salvando…" : "Salvar"}
              </Button>
              {save.isError && (
                <p role="alert" className="text-sm text-danger">
                  Falha ao salvar: {save.error.message}
                </p>
              )}
            </form>
          )}
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!deleting}
        onOpenChange={(open) => {
          if (!open && !remove.isPending) setDeleting(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Excluir regra?</DialogTitle>
            <DialogDescription>
              A regra {deleting?.name} deixará de ser avaliada. Confirme para
              excluir.
            </DialogDescription>
          </DialogHeader>
          <div className="flex gap-3">
            <Button
              variant="outline"
              disabled={remove.isPending}
              onClick={() => setDeleting(null)}
            >
              Cancelar
            </Button>
            <Button
              disabled={demo || remove.isPending}
              onClick={() => deleting && remove.mutate(deleting.id)}
            >
              Confirmar exclusão
            </Button>
          </div>
          {remove.isError && (
            <p role="alert" className="text-danger">
              Falha ao excluir: {remove.error.message}
            </p>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
function Message({ children }: { children: React.ReactNode }) {
  return (
    <p className="rounded border border-border bg-surface p-6 text-sm text-muted-foreground">
      {children}
    </p>
  );
}
export type APMProblem = {
  id: string;
  rule_id: string;
  service_id: string;
  name: string;
  severity: string;
  status: string;
  opened_at: string;
  updated_at: string;
  resolved_at?: string;
  value: number;
  threshold: number;
  metric: string;
  window_minutes: number;
  evidence: string[];
  example_trace_ids: string[];
  timeline: { at: string; state: string; value: number; summary: string }[];
};
const time = (value: string) => new Date(value).toLocaleString("pt-BR");
function stateLabel(state: string) {
  return state === "active"
    ? "Ativo"
    : state === "resolved"
      ? "Resolvido"
      : state === "unknown"
        ? "Dados insuficientes"
        : state;
}
function ProblemCard({
  problem: p,
  expanded,
  onOpen,
}: {
  problem: APMProblem;
  expanded: boolean;
  onOpen: () => void;
}) {
  const unknown = p.timeline?.at(-1)?.state === "unknown";
  return (
    <article className="rounded border border-border bg-surface p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="font-semibold">{p.name}</h2>
          <p className="mt-1 break-all text-xs text-muted-foreground">
            {p.service_id} · {p.severity} · {stateLabel(p.status)}
            {unknown ? " · Dados insuficientes na última avaliação" : ""}
          </p>
          <p className="mt-2 text-sm">
            Último valor registrado: {p.value.toLocaleString("pt-BR")}
            {p.metric === "error_rate" ? "%" : " ms"} · Limiar: {p.threshold} ·
            Janela: {p.window_minutes} min
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            Aberto: {time(p.opened_at)} · Atualizado: {time(p.updated_at)}
            {p.resolved_at ? " · Resolvido: " + time(p.resolved_at) : ""}
          </p>
        </div>
        <Button variant="outline" aria-expanded={expanded} onClick={onOpen}>
          {expanded ? "Recolher" : "Detalhes"}
        </Button>
      </div>
      {expanded && (
        <div className="mt-4 space-y-4 border-t border-border pt-4">
          <h3 className="text-sm font-semibold">Evidências da regra</h3>
          <ul className="list-inside list-disc space-y-2 text-xs">
            {p.evidence?.map((e, i) => (
              <li key={i} className="break-all">
                {e}
              </li>
            ))}
          </ul>
          <h3 className="text-sm font-semibold">Histórico de avaliação</h3>
          <ol className="space-y-3">
            {p.timeline?.map((event, i) => (
              <li
                key={event.at + ":" + i}
                className="border-l-2 border-primary pl-3 text-xs"
              >
                <p>
                  {time(event.at)} · {stateLabel(event.state)}
                </p>
                <p className="mt-1 text-muted-foreground">{event.summary}</p>
                {event.state !== "unknown" && (
                  <p className="mt-1">
                    Valor observado: {event.value.toLocaleString("pt-BR")}
                    {p.metric === "error_rate" ? "%" : " ms"}
                  </p>
                )}
              </li>
            ))}
          </ol>
          <h3 className="text-sm font-semibold">Traces de exemplo</h3>
          {p.example_trace_ids?.length ? (
            <ul className="space-y-2">
              {p.example_trace_ids.map((id) => (
                <li key={id} className="break-all font-mono text-xs">
                  <a
                    className="text-primary hover:underline"
                    href={"/traces/" + encodeURIComponent(id)}
                    aria-label={"Abrir trace " + id}
                  >
                    {id}
                  </a>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-xs text-muted-foreground">
              Nenhum trace de exemplo retido.
            </p>
          )}
        </div>
      )}
    </article>
  );
}
