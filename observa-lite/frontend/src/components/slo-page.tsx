import { ConfigurationForm, ConfigField } from "./configuration-form";
import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, request } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
type Policy = {
  id: string;
  name: string;
  service_id: string;
  kind: "availability" | "latency";
  target_percent: number;
  latency_threshold_ms?: number;
};
export type SLOEvaluation = Policy & {
  from: string;
  to: string;
  total: number;
  good: number;
  bad: number;
  success_percent: number | null;
  error_budget_remaining_percent: number | null;
  burn_rate: number | null;
  status: string;
  basis: string;
};
const windowFor = (minutes: number) => {
  const to = new Date();
  return {
    from: new Date(to.getTime() - minutes * 60000).toISOString(),
    to: to.toISOString(),
  };
};
const value = (n: number | null | undefined, suffix = "") =>
  n == null
    ? "—"
    : `${n.toLocaleString("pt-BR", { maximumFractionDigits: 3 })}${suffix}`;
const errorText = (e: unknown) =>
  e instanceof Error ? e.message : "Erro desconhecido";
export function SLOPage({
  demo = false,
  initialCreate = false,
}: {
  demo?: boolean;
  initialCreate?: boolean;
}) {
  const client = useQueryClient();
  const [minutes, setMinutes] = useState(60);
  const [window, setWindow] = useState(() => windowFor(60));
  const [editor, setEditor] = useState<Policy | null>(() =>
    initialCreate && !demo
      ? {
          id: `slo-${Date.now()}`,
          name: "",
          service_id: "",
          kind: "availability",
          target_percent: 99.5,
        }
      : null,
  );
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState<SLOEvaluation | null>(null);
  const query = useQuery({
    queryKey: ["slos", window],
    queryFn: () =>
      request<{ slos: SLOEvaluation[] }>(
        `/api/v1/slos?${new URLSearchParams(window)}`,
      ),
    enabled: !demo,
    retry: false,
  });
  const services = useQuery({
    queryKey: ["slo-services", window],
    queryFn: () => api.apmServices(window.from, window.to),
    enabled: !demo && editor !== null,
    retry: false,
  });
  const save = useMutation({
    mutationFn: (p: Policy) =>
      request("/api/v1/slos", { method: "POST", body: JSON.stringify(p) }),
    onSuccess: () => {
      setEditor(null);
      client.invalidateQueries({ queryKey: ["slos"] });
    },
  });
  const remove = useMutation({
    mutationFn: (id: string) =>
      request(`/api/v1/slos/${encodeURIComponent(id)}`, { method: "DELETE" }),
    onSuccess: () => {
      setDeleting(null);
      client.invalidateQueries({ queryKey: ["slos"] });
    },
  });
  function create() {
    save.reset();
    setEditing(false);
    setEditor({
      id: `slo-${Date.now()}`,
      name: "",
      service_id: "",
      kind: "availability",
      target_percent: 99.5,
    });
  }
  function submit(e: FormEvent) {
    e.preventDefault();
    if (!demo && editor) save.mutate(editor);
  }
  return (
    <div className="space-y-5">
      <header>
        <div className="text-xs uppercase text-primary">
          Confiabilidade / Objetivos
        </div>
        <h1 className="mt-1 text-2xl font-semibold">
          Objetivos de nível de serviço
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Avaliação da amostra de requisições retidas. Este resultado não
          representa disponibilidade por tempo de relógio nem dispara
          notificações.
        </p>
      </header>
      {demo ? (
        <p role="status" className="rounded border border-border p-4">
          Modo demonstração: consultas e alterações de SLO desativadas.
        </p>
      ) : (
        <>
          <div className="flex flex-wrap items-end gap-3">
            <ConfigField>
              Intervalo
              <select
                aria-label="Intervalo"
                className="ml-2 h-9 rounded border border-border bg-background px-3"
                value={minutes}
                onChange={(e) => setMinutes(Number(e.target.value))}
              >
                <option value={15}>15 minutos</option>
                <option value={60}>1 hora</option>
                <option value={360}>6 horas</option>
                <option value={1440}>24 horas</option>
              </select>
            </ConfigField>
            <Button
              onClick={() => setWindow(windowFor(minutes))}
              disabled={query.isFetching}
            >
              Atualizar
            </Button>
            <Button onClick={create}>Novo SLO</Button>
          </div>
          <p className="text-xs text-muted-foreground">
            Janela: {new Date(window.from).toLocaleString("pt-BR")} até{" "}
            {new Date(window.to).toLocaleString("pt-BR")}. Atualizar aplica o
            intervalo selecionado.
          </p>
          {query.isFetching && <p role="status">Carregando objetivos…</p>}
          {query.isError && (
            <div role="alert">
              Falha ao consultar: {errorText(query.error)}
              <Button className="ml-3" onClick={() => query.refetch()}>
                Tentar novamente
              </Button>
            </div>
          )}
          {query.isSuccess &&
            !query.isFetching &&
            (query.data.slos.length === 0 ? (
              <p className="rounded-lg border border-dashed border-border p-8 text-center">
                Nenhum SLO configurado. Crie um objetivo para avaliar um
                serviço.
              </p>
            ) : (
              <div className="overflow-x-auto rounded-lg border border-border bg-surface">
                <table className="w-full text-left text-xs">
                  <thead>
                    <tr>
                      {[
                        "Objetivo / Serviço",
                        "Tipo / Meta",
                        "Amostra",
                        "Resultado",
                        "Orçamento restante",
                        "Burn rate",
                        "Ações",
                      ].map((h) => (
                        <th className="p-3" key={h}>
                          {h}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {query.data.slos.map((s) => (
                      <tr
                        className="border-t border-border align-top"
                        key={s.id}
                      >
                        <td className="p-3">
                          <strong>{s.name}</strong>
                          <div className="mt-1 break-all text-muted-foreground">
                            {s.service_id}
                          </div>
                          <details className="mt-2">
                            <summary>Base da avaliação</summary>
                            {s.basis}
                          </details>
                        </td>
                        <td className="p-3">
                          {s.kind === "latency"
                            ? `Latência ≤ ${value(s.latency_threshold_ms, " ms")}`
                            : "Disponibilidade por requisição"}
                          <div>Meta: {value(s.target_percent, "%")}</div>
                        </td>
                        <td className="p-3">
                          Total: {s.total}
                          <div>Boas: {s.good}</div>
                          <div>Ruins: {s.bad}</div>
                        </td>
                        <td className="p-3">
                          {s.status === "unknown"
                            ? "Sem dados"
                            : s.status === "met"
                              ? "Atendido"
                              : "Violado"}
                          <div>{value(s.success_percent, "%")}</div>
                        </td>
                        <td className="p-3">
                          {value(s.error_budget_remaining_percent, " p.p.")}
                        </td>
                        <td className="p-3">
                          {value(s.burn_rate, "×")}
                          {s.target_percent === 100 && (
                            <div className="text-muted-foreground">
                              Meta 100%: sem orçamento de erro
                            </div>
                          )}
                        </td>
                        <td className="p-3">
                          <Button
                            variant="outline"
                            onClick={() => {
                              save.reset();
                              setEditing(true);
                              setEditor({
                                id: s.id,
                                name: s.name,
                                service_id: s.service_id,
                                kind: s.kind,
                                target_percent: s.target_percent,
                                ...(s.latency_threshold_ms !== undefined
                                  ? {
                                      latency_threshold_ms:
                                        s.latency_threshold_ms,
                                    }
                                  : {}),
                              });
                            }}
                          >
                            Editar
                          </Button>
                          <Button
                            variant="outline"
                            className="ml-2"
                            onClick={() => {
                              remove.reset();
                              setDeleting(s);
                            }}
                          >
                            Excluir
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ))}
        </>
      )}
      <Dialog
        open={!demo && editor !== null}
        onOpenChange={(open) => {
          if (!open && !save.isPending) setEditor(null);
        }}
      >
        <DialogContent>
          <DialogTitle>{editing ? "Editar SLO" : "Novo SLO"}</DialogTitle>
          <DialogDescription>
            Defina a proporção de requisições boas na janela consultada.
          </DialogDescription>
          {editor && (
            <ConfigurationForm className="space-y-3" onSubmit={submit}>
              <ConfigField className="block text-sm">
                Nome
                <Input
                  aria-label="Nome"
                  value={editor.name}
                  maxLength={256}
                  required
                  onChange={(e) =>
                    setEditor({ ...editor, name: e.target.value })
                  }
                />
              </ConfigField>
              <ConfigField className="block text-sm">
                Serviço
                <select
                  aria-label="Serviço"
                  className="mt-1 block w-full rounded border border-border bg-background p-2"
                  required
                  value={editor.service_id}
                  onChange={(e) =>
                    setEditor({ ...editor, service_id: e.target.value })
                  }
                >
                  <option value="">Selecione um serviço</option>
                  {editor.service_id &&
                    !services.data?.services.some(
                      (s) => s.id === editor.service_id,
                    ) && (
                      <option value={editor.service_id}>
                        {editor.service_id}
                      </option>
                    )}
                  {services.data?.services.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} ({s.namespace || "sem namespace"})
                    </option>
                  ))}
                </select>
              </ConfigField>
              {services.isFetching && <p role="status">Carregando serviços…</p>}
              {services.isError && (
                <p role="alert">
                  Falha ao carregar serviços: {errorText(services.error)}{" "}
                  <button type="button" onClick={() => services.refetch()}>
                    Tentar novamente
                  </button>
                </p>
              )}
              <ConfigField className="block text-sm">
                Tipo
                <select
                  aria-label="Tipo"
                  className="ml-2 rounded border border-border bg-background p-2"
                  value={editor.kind}
                  onChange={(e) =>
                    setEditor({
                      ...editor,
                      kind: e.target.value as Policy["kind"],
                    })
                  }
                >
                  <option value="availability">Disponibilidade</option>
                  <option value="latency">Latência</option>
                </select>
              </ConfigField>
              <ConfigField className="block text-sm">
                Meta (%)
                <Input
                  aria-label="Meta (%)"
                  type="number"
                  min="0.000001"
                  max="100"
                  step="any"
                  required
                  value={editor.target_percent}
                  onChange={(e) =>
                    setEditor({
                      ...editor,
                      target_percent: Number(e.target.value),
                    })
                  }
                />
              </ConfigField>
              {editor.kind === "latency" && (
                <ConfigField className="block text-sm">
                  Limite de latência (ms)
                  <Input
                    aria-label="Limite de latência (ms)"
                    type="number"
                    min="0.000001"
                    step="any"
                    required
                    value={editor.latency_threshold_ms ?? ""}
                    onChange={(e) =>
                      setEditor({
                        ...editor,
                        latency_threshold_ms: Number(e.target.value),
                      })
                    }
                  />
                </ConfigField>
              )}
              {save.isError && (
                <p role="alert">Falha ao salvar: {errorText(save.error)}</p>
              )}
              <Button
                type="submit"
                disabled={save.isPending || !editor.service_id}
              >
                {save.isPending ? "Salvando…" : "Salvar"}
              </Button>
              <Button
                type="button"
                variant="outline"
                className="ml-2"
                disabled={save.isPending}
                onClick={() => setEditor(null)}
              >
                Cancelar
              </Button>
            </ConfigurationForm>
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
          <DialogTitle>Excluir SLO?</DialogTitle>
          <DialogDescription>
            Excluir o objetivo {deleting?.name}? A telemetria recebida será
            preservada.
          </DialogDescription>
          {remove.isError && (
            <p role="alert">Falha ao excluir: {errorText(remove.error)}</p>
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
