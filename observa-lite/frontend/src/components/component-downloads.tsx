import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Download,
  Network,
  Server,
  Database,
  Monitor,
  type LucideIcon,
} from "lucide-react";
import { request, readToken } from "@/lib/api";
const topology: [LucideIcon, string, string][] = [
  [Monitor, "Vetra Web", "Interface e acesso"],
  [Server, "Vetra Server", "API, consultas e ingestão"],
  [Database, "Banco de dados", "Armazenamento interno"],
  [Network, "Vetra Relay", "Gateway TLS e telemetria"],
  [Server, "Vetra Agent", "Descoberta de hosts"],
];
type Component = {
  id: string;
  name: string;
  platform: string;
  bytes: number;
  sha256: string;
  url: string;
};
export function ComponentDownloads() {
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");
  const q = useQuery({
    queryKey: ["downloads"],
    queryFn: () =>
      request<{ components: Component[]; storage: Record<string, unknown> }>(
        "/api/v1/downloads",
      ),
  });
  const download = async (c: Component) => {
    setBusy(c.id);
    setError("");
    try {
      const r = await fetch(c.url, {
        headers: readToken() ? { "X-API-Key": readToken() } : {},
      });
      if (!r.ok) throw Error("Não foi possível baixar o componente");
      const blob = await r.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = c.id;
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Falha no download");
    } finally {
      setBusy("");
    }
  };
  return (
    <div className="space-y-5">
      <h1 className="text-2xl font-semibold">Componentes e downloads</h1>
      <p className="text-sm text-muted-foreground">
        Instale os componentes no host ou separe as funções em containers. As
        credenciais de ingestão são configuradas na instalação; não estão
        embutidas nos arquivos.
      </p>
      {(q.error || error) && <p role="alert">{error || q.error?.message}</p>}
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
        {topology.map(([Icon, name, description]) => (
          <section
            key={String(name)}
            className="space-y-3 border bg-surface p-4"
          >
            {<Icon className="text-emerald-400" />}
            <h2 className="font-semibold">{String(name)}</h2>
            <p className="text-xs text-muted-foreground">
              {String(description)}
            </p>
          </section>
        ))}
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        {q.data?.components.map((c) => (
          <section key={c.id} className="space-y-3 border bg-surface p-5">
            <div className="flex justify-between gap-4">
              <div>
                <h2 className="font-semibold">{c.name}</h2>
                <p className="text-xs text-muted-foreground">
                  {c.platform} · {(c.bytes / 1048576).toFixed(1)} MiB
                </p>
              </div>
              <button
                disabled={!!busy}
                onClick={() => void download(c)}
                className="inline-flex items-center gap-2 border border-emerald-800 bg-emerald-950 px-3 py-2 text-sm text-emerald-100 disabled:opacity-40"
              >
                <Download size={16} />
                {busy === c.id ? "Baixando…" : "Baixar"}
              </button>
            </div>
            <p className="break-all font-mono text-xs text-muted-foreground">
              SHA-256: {c.sha256}
            </p>
          </section>
        ))}
      </div>
      {q.data && !q.data.components.length && (
        <p className="border p-4 text-sm">
          Nenhum binário foi empacotado neste servidor. A imagem Docker nova
          inclui os downloads; na instalação por código, execute
          scripts/build-components.sh e reinicie com VETRA_DOWNLOAD_DIR
          apontando para a pasta gerada.
        </p>
      )}
      <section className="space-y-3 border bg-surface p-5 text-sm">
        <h2 className="font-semibold">Como usar os arquivos</h2>
        <p>
          Vetra Agent: execute o arquivo com o subcomando agent, endereço do
          servidor e OBSERVA_AGENT_TOKEN. Vetra Relay: gateway TLS para agentes,
          OTLP e RUM. Vetra Relay Web: proxy opcional de injeção HTML,
          configurado em RUM → Instalação.
        </p>
        <pre className="overflow-auto bg-black p-3 text-xs">{`chmod +x vetra-agent-linux-amd64\nexport OBSERVA_AGENT_TOKEN='TOKEN-DE-INGESTAO'\n./vetra-agent-linux-amd64 agent --server https://coletor.empresa`}</pre>
        <p className="text-xs text-muted-foreground">
          Banco SQLite é local ao servidor. Para separar banco e API, use
          PostgreSQL no perfil de componentes. Isso não torna o servidor
          automaticamente distribuído.
        </p>
      </section>
    </div>
  );
}
