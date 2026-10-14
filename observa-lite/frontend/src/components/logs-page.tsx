import { useMemo, useState, type FormEvent } from "react";
import { useMutation } from "@tanstack/react-query";
import { Database, Filter, ScrollText, Search, ShieldCheck } from "lucide-react";
import { api, type DataSource, type LogQueryResult } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

export function LogsPage({ sources, demo }: { sources: DataSource[]; demo: boolean }) {
  const loki = useMemo(() => sources.filter((s) => s.kind === "loki"), [sources]);
  const [source, setSource] = useState("");
  const [expr, setExpr] = useState('{service="api"}');
  const [minutes, setMinutes] = useState(60);
  const selected = loki.some((s) => s.id === source) ? source : (loki[0]?.id ?? "");
  const request = useMutation({ mutationFn: async () => api.logs(selected, expr.trim(), minutes, 100) });
  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (demo || !selected) return;
    request.mutate();
  }
  const result: LogQueryResult | undefined = request.data;
  return <div className="space-y-5">
    <header><div className="mb-1 text-[11px] font-semibold uppercase text-primary">Explorar / Logs</div>
      <h1 className="text-2xl font-semibold">Logs</h1>
      <p className="mt-1 text-sm text-muted-foreground">Consulta LogQL diretamente no Loki. O Observa Lite não armazena nem indexa o conteúdo.</p>
    </header>
    <div className="rounded-lg border border-border bg-surface p-4">
      <div className="mb-4 flex items-start gap-3 rounded-md border border-primary/25 bg-primary/5 p-3 text-xs text-muted-foreground">
        <Database className="size-4 shrink-0 text-primary" />
        <p>Consulta limitada a 24 horas e 100 linhas na interface (máximo de 200 pela API). Os logs permanecem exclusivamente no armazenamento Loki, com retenção administrada por ele.</p>
      </div>
      {demo ? <p className="rounded-md border border-warning/30 bg-warning/10 p-3 text-sm text-warning">Modo demonstração: consultas reais desativadas.</p> :
      loki.length === 0 ? <div className="rounded-md border border-warning/30 bg-warning/10 p-4 text-sm text-warning">Nenhum Loki configurado. Vá em Data Sources e cadastre uma fonte do tipo Loki antes de consultar os logs.</div> :
      <form onSubmit={submit} className="grid gap-4 lg:grid-cols-[1.1fr_1.6fr_150px_auto] lg:items-end">
        <div><Label>Fonte de dados Loki</Label><Select value={selected} onValueChange={setSource}><SelectTrigger className="mt-2"><SelectValue placeholder="Selecionar Loki"/></SelectTrigger><SelectContent>{loki.map((s) => <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>)}</SelectContent></Select></div>
        <div><Label htmlFor="logql">Consulta LogQL</Label><Input id="logql" className="mt-2 font-mono text-xs" value={expr} onChange={(e) => setExpr(e.target.value)} maxLength={512} placeholder={'{service="api"} |= "error"'} required/></div>
        <div><Label>Intervalo</Label><Select value={String(minutes)} onValueChange={(v) => setMinutes(Number(v))}><SelectTrigger className="mt-2"><SelectValue/></SelectTrigger><SelectContent><SelectItem value="15">15 minutos</SelectItem><SelectItem value="60">1 hora</SelectItem><SelectItem value="360">6 horas</SelectItem><SelectItem value="1440">24 horas</SelectItem></SelectContent></Select></div>
        <Button type="submit" disabled={request.isPending}><Search className="size-4"/>{request.isPending ? "Consultando…" : "Consultar"}</Button>
      </form>}
    </div>
    {request.isError && <div className="rounded-lg border border-danger/40 bg-danger/10 p-4 text-sm text-danger">Falha na consulta: {request.error instanceof Error ? request.error.message : "erro não identificado"}. Confira o seletor LogQL, o Loki e se destinos de rede privados foram explicitamente autorizados.</div>}
    {result && <section className="overflow-hidden rounded-lg border border-border bg-surface">
      <div className="flex items-center justify-between border-b border-border p-4"><div className="flex items-center gap-2"><ScrollText className="size-4 text-primary"/><h2 className="text-sm font-semibold">Resultados</h2></div><div className="text-xs text-muted-foreground">{result.logs.length} linhas · últimas {result.minutes} min</div></div>
      {!result.logs.length ? <div className="p-10 text-center text-sm text-muted-foreground">Nenhuma linha encontrada. Ajuste o seletor ou o intervalo.</div> :
      <div className="max-h-[65vh] overflow-auto">{result.logs.map((l,i)=><div key={i} className="grid gap-2 border-b border-border/70 px-4 py-3 text-xs lg:grid-cols-[170px_1fr]">
        <time className="shrink-0 font-mono text-muted-foreground">{new Date(l.at).toLocaleString("pt-BR")}</time>
        <div className="min-w-0"><div className="mb-1 flex flex-wrap gap-1">{Object.entries(l.labels||{}).filter(([,v])=>typeof v==="string").slice(0,8).map(([k,v])=><span key={k} className="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">{k}={String(v)}</span>)}</div><pre className="whitespace-pre-wrap break-words font-mono text-xs leading-5">{l.line}</pre></div>
      </div>)}</div>}
    </section>}
    {!result && !request.isPending && <div className="flex min-h-44 flex-col items-center justify-center rounded-lg border border-dashed border-border p-6 text-center"><ShieldCheck className="mb-3 size-8 text-primary"/><h3 className="text-sm font-semibold">Consulta sob demanda</h3><p className="mt-1 max-w-lg text-xs text-muted-foreground">Para economizar recursos, os logs não são consultados automaticamente nesta tela. Apenas fontes Loki configuradas podem ser pesquisadas.</p></div>}
    <footer className="flex items-center gap-2 text-xs text-muted-foreground"><Filter className="size-3.5"/> Labels recomendados: ambiente, serviço, nível e host. Evite trace_id, PID e request_id como labels indexados.</footer>
  </div>;
}
