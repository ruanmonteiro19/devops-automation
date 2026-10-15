import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Database, Download } from "lucide-react";
import { request, readToken } from "@/lib/api";
export function StoragePage() {
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const q = useQuery({
    queryKey: ["storage"],
    queryFn: () => request<Record<string, unknown>>("/api/v1/storage"),
    refetchInterval: 15000,
  });
  const me = useQuery({
    queryKey: ["auth-session"],
    queryFn: () => request<{ user: { role: string } }>("/api/v1/auth/session"),
  });
  const admin = !!readToken() || me.data?.user.role === "admin";
  const backup = async () => {
    setBusy(true);
    setError("");
    try {
      const r = await fetch("/api/v1/storage/export", {
        method: "POST",
        headers: {
          "X-CSRF-Token": sessionStorage.getItem("vetra-csrf") || "",
          ...(readToken() ? { "X-API-Key": readToken() } : {}),
        },
      });
      if (!r.ok) throw Error("Não foi possível exportar o estado");
      const url = URL.createObjectURL(await r.blob());
      const a = document.createElement("a");
      a.href = url;
      a.download = "vetra-state-export.json";
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Falha na exportação");
    } finally {
      setBusy(false);
    }
  };
  const data = q.data;
  return (
    <div className="space-y-5">
      <header className="flex justify-between gap-4">
        <h1 className="flex items-center gap-3 text-2xl font-semibold">
          <Database className="text-emerald-400" />
          Armazenamento
        </h1>
        {admin && (
          <button
            className="flex items-center gap-2 border border-emerald-800 bg-emerald-950 px-3 py-2 text-sm text-emerald-100 disabled:opacity-40"
            disabled={busy}
            onClick={() => void backup()}
          >
            <Download size={16} />
            {busy ? "Exportando…" : "Exportar backup"}
          </button>
        )}
      </header>
      {(error || q.error) && <p role="alert">{error || q.error?.message}</p>}
      <section className="space-y-4 border bg-surface p-5">
        <h2 className="font-semibold">
          Banco ativo: {String(data?.["engine"] ?? "Aguardando")}
        </h2>
        <p className="text-sm text-muted-foreground">
          {data?.["incremental"]
            ? "Gravação incremental e payloads comprimidos. Somente registros alterados são enviados ao banco."
            : "Modo JSON legado: o estado é regravado em cada persistência."}
        </p>
        {data?.["engine"] === "sqlite" && (
          <p className="text-sm">
            Arquivos locais, incluindo WAL e SHM:{" "}
            {(Number(data["disk_bytes"] ?? 0) / 1048576).toFixed(2)} MiB.
          </p>
        )}
        <div className="grid gap-3 md:grid-cols-3">
          {[
            ["Spans", "spans"],
            ["Logs", "logs"],
            ["Métricas", "metrics"],
            ["Eventos RUM", "rum_events"],
            ["Blocos de replay", "replay_chunks"],
          ].map(([label, key]) => (
            <div key={key} className="border bg-black p-4">
              <p className="text-xs text-muted-foreground">{label}</p>
              <p className="mt-2 text-xl font-semibold">
                {Number(data?.[key!] ?? 0).toLocaleString("pt-BR")}
              </p>
            </div>
          ))}
        </div>
      </section>
      <section className="space-y-3 border bg-surface p-5 text-sm">
        <h2 className="font-semibold">Retenção e limites</h2>
        <p>
          Janela de {String(data?.["retention_hours"] ?? 24)} horas, com
          manutenção periódica e limites por sinal. Os limites também podem
          remover os registros mais antigos antes de 24 horas.
        </p>
        <p>
          Até {String(data?.["span_limit"] ?? 20000)} spans,{" "}
          {String(data?.["log_limit"] ?? 10000)} logs,{" "}
          {String(data?.["metric_limit"] ?? 50000)} pontos de métricas,{" "}
          {String(data?.["rum_event_limit"] ?? 15000)} eventos RUM e 25 MiB de
          replay. Há limites adicionais de bytes nos sinais.
        </p>
        <p className="text-muted-foreground">
          Consultas usam a janela recente em memória. A Vetra não precisa de
          Loki ou Tempo para armazenar esses sinais. Não há armazenamento
          histórico distribuído, Parquet ou consultas DQL nesta versão.
        </p>
      </section>
      <section className="space-y-3 border bg-surface p-5 text-sm">
        <h2 className="font-semibold">Separar o banco</h2>
        <p>
          Use o perfil de componentes para PostgreSQL, API e interface em
          containers separados. A configuração do banco é feita no ambiente do
          servidor. Antes de migrar uma instalação SQLite, exporte o estado e
          use o arquivo na migração para preservar as contas e a telemetria.
        </p>
        <p className="text-muted-foreground">
          O backup inclui contas com hashes de senha, sessões e dados retidos.
          Guarde-o com acesso administrativo. Não contém as variáveis de
          ambiente com tokens e credenciais; mantenha também o backup do seu
          .env.
        </p>
      </section>
    </div>
  );
}
