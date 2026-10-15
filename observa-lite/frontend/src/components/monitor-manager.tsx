import { useState, type FormEvent } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Plus,
  Pencil,
  Copy,
  Pause,
  Play,
  Trash2,
  Globe2,
  Search,
  Download,
  CheckCircle2,
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
import { api, type MonitorTemplate } from "@/lib/api";
import { downloadJSON } from "@/lib/workspace";
import { WorkspaceHeading } from "./workspace-experience";
const blank = (): MonitorTemplate => ({
  id: crypto.randomUUID(),
  name: "",
  kind: "http",
  target: "",
  interval_seconds: 60,
  timeout_seconds: 5,
  expected_status: 200,
  enabled: true,
  last_run: "",
  last_value: 0,
  last_error: "",
  status: "unknown",
});
export function MonitorManager({
  items,
  demo,
  initialCreate = false,
}: {
  items: MonitorTemplate[];
  demo: boolean;
  initialCreate?: boolean;
}) {
  const client = useQueryClient();
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const [draft, setDraft] = useState<MonitorTemplate | null>(() =>
    initialCreate && !demo ? blank() : null,
  );
  const [deleting, setDeleting] = useState<MonitorTemplate | null>(null);
  const save = useMutation({
    mutationFn: api.createTemplate,
    onSuccess: () => {
      setDraft(null);
      void client.invalidateQueries({ queryKey: ["snapshot"] });
      toast.success("Monitor salvo");
    },
    onError: (e) => toast.error(e.message),
  });
  const remove = useMutation({
    mutationFn: api.deleteTemplate,
    onSuccess: () => {
      setDeleting(null);
      void client.invalidateQueries({ queryKey: ["snapshot"] });
      toast.success("Monitor excluído");
    },
    onError: (e) => toast.error(e.message),
  });
  const filtered = items.filter(
    (item) =>
      (status === "all" ||
        (status === "enabled" ? item.enabled : !item.enabled)) &&
      `${item.name} ${item.target} ${item.kind}`
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  function edit(item: MonitorTemplate) {
    save.reset();
    setDraft({ ...item });
  }
  function submit(e: FormEvent) {
    e.preventDefault();
    if (draft && !demo) save.mutate(draft);
  }
  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <WorkspaceHeading
          eyebrow="Confiabilidade / Monitores"
          title="Cada verificação, sob controle."
          description="Configure verificações HTTP e TCP, ajuste intervalos e pause monitores quando precisar."
        />
        <Button disabled={demo} onClick={() => edit(blank())}>
          <Plus className="size-4" />
          Criar monitor
        </Button>
      </div>
      <div className="grid gap-3 sm:grid-cols-3">
        {[
          ["Monitores", items.length],
          ["Em execução", items.filter((i) => i.enabled).length],
          ["Pausados", items.filter((i) => !i.enabled).length],
        ].map(([label, count]) => (
          <div key={label} className="rounded-xl border bg-surface p-5">
            <p className="text-xs text-muted-foreground">{label}</p>
            <p className="mt-2 text-3xl font-semibold">{count}</p>
          </div>
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative max-w-sm flex-1">
          <Search className="absolute left-3 top-3 size-4 text-muted-foreground" />
          <Input
            aria-label="Buscar monitores"
            placeholder="Buscar nome ou destino…"
            className="pl-9"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <select
          aria-label="Estado dos monitores"
          className="h-10 rounded-lg border bg-surface px-3 text-sm"
          value={status}
          onChange={(e) => setStatus(e.target.value)}
        >
          <option value="all">Todos os monitores</option>
          <option value="enabled">Em execução</option>
          <option value="paused">Pausados</option>
        </select>
        <Button
          variant="outline"
          onClick={() => downloadJSON("observa-monitores.json", filtered)}
        >
          <Download className="size-4" />
          Exportar
        </Button>
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        {filtered.map((item) => (
          <article key={item.id} className="rounded-xl border bg-surface p-5">
            <div className="flex items-start gap-3">
              <div className="grid size-10 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
                <Globe2 className="size-5" />
              </div>
              <div className="min-w-0 flex-1">
                <h2 className="font-semibold">{item.name}</h2>
                <p className="mt-1 break-all font-mono text-xs text-muted-foreground">
                  {item.target}
                </p>
              </div>
              <span className="rounded-full border px-2 py-1 text-[10px] font-medium">
                {item.enabled ? "ATIVO" : "PAUSADO"}
              </span>
            </div>
            <div className="my-5 grid grid-cols-3 gap-3 rounded-lg bg-background/60 p-3 text-xs">
              <div>
                <p className="text-muted-foreground">Protocolo</p>
                <p className="mt-1 uppercase">{item.kind}</p>
              </div>
              <div>
                <p className="text-muted-foreground">Intervalo</p>
                <p className="mt-1">{item.interval_seconds}s</p>
              </div>
              <div>
                <p className="text-muted-foreground">Timeout</p>
                <p className="mt-1">{item.timeout_seconds}s</p>
              </div>
            </div>
            <p className="mb-4 text-xs text-muted-foreground">
              {item.last_run && !item.last_run.startsWith("0001")
                ? `Última verificação: ${new Date(item.last_run).toLocaleString("pt-BR")} · ${item.status}`
                : "Aguardando primeira verificação"}
            </p>
            {item.last_error && (
              <p className="mb-4 break-words text-xs text-danger">
                {item.last_error}
              </p>
            )}
            <div className="flex flex-wrap gap-2 border-t pt-4">
              <Button
                variant="outline"
                size="sm"
                disabled={demo || save.isPending}
                onClick={() => edit(item)}
              >
                <Pencil className="size-3.5" />
                Editar
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={demo || save.isPending}
                onClick={() =>
                  edit({
                    ...blank(),
                    name: `${item.name} (cópia)`,
                    kind: item.kind,
                    target: item.target,
                    interval_seconds: item.interval_seconds,
                    timeout_seconds: item.timeout_seconds,
                    expected_status: item.expected_status,
                  })
                }
              >
                <Copy className="size-3.5" />
                Duplicar
              </Button>
              <Button
                variant="ghost"
                size="sm"
                disabled={demo || save.isPending}
                onClick={() => save.mutate({ ...item, enabled: !item.enabled })}
              >
                {item.enabled ? (
                  <Pause className="size-3.5" />
                ) : (
                  <Play className="size-3.5" />
                )}
                {item.enabled ? "Pausar" : "Retomar"}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                className="ml-auto text-danger"
                aria-label={`Excluir ${item.name}`}
                disabled={demo || save.isPending}
                onClick={() => {
                  remove.reset();
                  setDeleting(item);
                }}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
          </article>
        ))}
      </div>
      {!filtered.length && (
        <div className="rounded-xl border border-dashed p-10 text-center">
          <CheckCircle2 className="mx-auto mb-4 text-primary" />
          <h2 className="font-semibold">Nenhum monitor nesta seleção</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Crie uma verificação ou ajuste os filtros.
          </p>
        </div>
      )}
      <Dialog
        open={draft !== null}
        onOpenChange={(open) => {
          if (!open && !save.isPending) setDraft(null);
        }}
      >
        <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>
              {items.some((i) => i.id === draft?.id)
                ? "Editar monitor"
                : "Criar monitor"}
            </DialogTitle>
            <DialogDescription>
              Defina o destino, a frequência e o resultado esperado.
            </DialogDescription>
          </DialogHeader>
          {draft && (
            <form onSubmit={submit} className="space-y-4">
              <label className="block text-sm">
                Nome
                <Input
                  required
                  maxLength={256}
                  value={draft.name}
                  onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                />
              </label>
              <label className="block text-sm">
                Protocolo
                <select
                  aria-label="Protocolo do monitor"
                  className="mt-2 h-10 w-full rounded-lg border bg-background px-3"
                  value={draft.kind}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      kind: e.target.value as "http" | "tcp",
                    })
                  }
                >
                  <option value="http">HTTP / HTTPS</option>
                  <option value="tcp">TCP</option>
                </select>
              </label>
              <label className="block text-sm">
                Destino
                <Input
                  required
                  maxLength={1024}
                  placeholder={
                    draft.kind === "http"
                      ? "https://api.exemplo/health"
                      : "servidor.exemplo:443"
                  }
                  value={draft.target}
                  onChange={(e) =>
                    setDraft({ ...draft, target: e.target.value })
                  }
                />
              </label>
              <div className="grid grid-cols-2 gap-4">
                <label className="text-sm">
                  Intervalo (segundos)
                  <Input
                    type="number"
                    min={10}
                    max={86400}
                    required
                    value={draft.interval_seconds}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        interval_seconds: Number(e.target.value),
                      })
                    }
                  />
                </label>
                <label className="text-sm">
                  Timeout (segundos)
                  <Input
                    type="number"
                    min={1}
                    max={30}
                    required
                    value={draft.timeout_seconds}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        timeout_seconds: Number(e.target.value),
                      })
                    }
                  />
                </label>
              </div>
              {draft.kind === "http" && (
                <label className="block text-sm">
                  Status HTTP esperado
                  <Input
                    type="number"
                    min={100}
                    max={599}
                    required
                    value={draft.expected_status}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        expected_status: Number(e.target.value),
                      })
                    }
                  />
                </label>
              )}
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={draft.enabled}
                  onChange={(e) =>
                    setDraft({ ...draft, enabled: e.target.checked })
                  }
                />
                Monitor habilitado
              </label>
              <p className="rounded-lg bg-muted p-3 text-xs leading-5 text-muted-foreground">
                O servidor executa a verificação. Para destinos privados, o
                administrador precisa habilitar OBSERVA_ALLOW_PRIVATE_TARGETS no
                ambiente do servidor.
              </p>
              {save.isError && (
                <p role="alert" className="text-sm text-danger">
                  {save.error.message}
                </p>
              )}
              <div className="flex justify-end gap-2">
                <Button
                  type="button"
                  variant="outline"
                  disabled={save.isPending}
                  onClick={() => setDraft(null)}
                >
                  Cancelar
                </Button>
                <Button disabled={demo || save.isPending}>
                  {save.isPending ? "Salvando…" : "Salvar monitor"}
                </Button>
              </div>
            </form>
          )}
        </DialogContent>
      </Dialog>
      <Dialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open && !remove.isPending) setDeleting(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Excluir monitor?</DialogTitle>
            <DialogDescription>
              O monitor {deleting?.name} deixará de executar novas verificações.
              Esta ação não pode ser desfeita.
            </DialogDescription>
          </DialogHeader>
          {remove.isError && <p role="alert">{remove.error.message}</p>}
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              disabled={remove.isPending}
              onClick={() => setDeleting(null)}
            >
              Cancelar
            </Button>
            <Button
              variant="destructive"
              disabled={demo || remove.isPending}
              onClick={() => {
                if (deleting) remove.mutate(deleting.id);
              }}
            >
              Excluir monitor
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
