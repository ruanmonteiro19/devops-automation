import { newID } from "@/lib/id";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { request } from "../lib/api";
import {
  BookOpen,
  Play,
  Plus,
  Save,
  Trash2,
  Database,
  Workflow,
  ShieldCheck,
  Globe2,
  Terminal,
} from "lucide-react";

type Cell = { id: string; kind: string; content: string };
type Step = {
  id: string;
  name: string;
  kind: string;
  query?: string;
  target_id?: string;
  min_rows?: number;
};
type Obj = {
  id: string;
  name: string;
  kind: string;
  enabled: boolean;
  cells?: Cell[];
  steps?: Step[];
  engine?: string;
  credential_env?: string;
  interval_seconds?: number;
  packages?: { name: string; version: string; ecosystem: string }[];
};
type Run = {
  id: string;
  object_id: string;
  status: string;
  started_at: string;
  error?: string;
  result: Record<string, unknown>;
};
type Row = Record<string, unknown>;
const id = () => newID();
const input =
  "w-full rounded-sm border border-emerald-900 bg-black px-3 py-2 text-sm text-zinc-100 focus:outline-emerald-500";
const button =
  "inline-flex items-center gap-2 rounded-sm border border-emerald-800 bg-emerald-950 px-3 py-2 text-sm text-emerald-100 hover:bg-emerald-900 disabled:opacity-40";
const box = "rounded-sm border border-zinc-800 bg-zinc-950 p-5 space-y-4";
const labels: Record<string, string> = {
  notebooks: "Notebooks",
  workflows: "Workflows",
  databases: "Databases",
  vulnerabilidades: "Vulnerabilidades",
  frontends: "Frontends descobertos",
  mcp: "Assistente via MCP",
};
function Table({ rows }: { rows: Row[] }) {
  const keys = [...new Set(rows.flatMap(Object.keys))].slice(0, 16);
  if (!rows.length)
    return (
      <p className="text-sm text-zinc-400">Nenhum registro nesta consulta.</p>
    );
  return (
    <div className="overflow-auto max-h-96">
      <table className="w-full text-sm">
        <thead>
          <tr>
            {keys.map((k) => (
              <th
                className="text-left p-2 bg-emerald-950 text-emerald-200"
                key={k}
              >
                {k}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i} className="border-b border-zinc-800">
              {keys.map((k) => (
                <td key={k} className="p-2 max-w-80 break-words">
                  {typeof r[k] === "object"
                    ? JSON.stringify(r[k])
                    : String(r[k] ?? "—")}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
export function WorkspaceStudio({
  path,
  demo,
}: {
  path: string;
  demo: boolean;
}) {
  const qc = useQueryClient();
  const kind = (
    {
      notebooks: "notebook",
      workflows: "workflow",
      databases: "database",
      vulnerabilidades: "software",
    } as Record<string, string>
  )[path];
  const data = useQuery({
    queryKey: ["workspace"],
    queryFn: () =>
      request<{ objects: Obj[]; runs: Run[] }>("/api/v1/workspace"),
    enabled: !demo,
    refetchInterval: 3000,
  });
  const inventory = useQuery({
    queryKey: ["workspace-inventory", path],
    queryFn: () =>
      request<{ frontends?: Row[]; databases?: Row[] }>(
        path === "frontends"
          ? "/api/v1/frontends/discovered"
          : "/api/v1/databases/inventory",
      ),
    enabled: !demo && ["frontends", "databases"].includes(path),
    refetchInterval: 15000,
  });
  const [editing, setEditing] = useState<Obj | null>(null);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [results, setResults] = useState<Record<string, Row[]>>({});
  const [hostID, setHostID] = useState("");
  const [origin, setOrigin] = useState("");
  const [candidate, setCandidate] = useState<Row | null>(null);
  const objects = data.data?.objects ?? [];
  const runs = data.data?.runs ?? [];
  const perform = async (fn: () => Promise<unknown>, success: string) => {
    setBusy(true);
    setMessage("");
    try {
      await fn();
      setMessage(success);
      await qc.invalidateQueries({ queryKey: ["workspace"] });
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Falha na operação");
    } finally {
      setBusy(false);
    }
  };
  const create = () => {
    setResults({});
    setEditing({
      id: id(),
      name: "",
      kind: kind!,
      enabled: false,
      ...(kind === "notebook"
        ? {
            cells: [
              {
                id: id(),
                kind: "query",
                content: "fetch hosts | fields name, os, last_seen | limit 100",
              },
            ],
          }
        : {}),
      ...(kind === "workflow"
        ? {
            steps: [
              {
                id: id(),
                name: "Consultar hosts",
                kind: "query",
                query: "fetch hosts | limit 100",
              },
            ],
            interval_seconds: 0,
          }
        : {}),
      ...(kind === "database"
        ? { engine: "postgresql", credential_env: "OBSERVA_DB_PRINCIPAL_DSN" }
        : {}),
      ...(kind === "software"
        ? { packages: [{ name: "", version: "", ecosystem: "npm" }] }
        : {}),
    });
  };
  const save = () =>
    editing &&
    perform(
      () =>
        request("/api/v1/workspace", {
          method: "POST",
          body: JSON.stringify(editing),
        }),
      "Configuração salva.",
    );
  const run = (o: Obj) =>
    perform(
      () =>
        request(`/api/v1/workspace/${encodeURIComponent(o.id)}/run`, {
          method: "POST",
        }),
      "Execução iniciada. Acompanhe o resultado no histórico.",
    );
  const patchStep = (index: number, patch: Partial<Step>) =>
    editing &&
    setEditing({
      ...editing,
      steps: (editing.steps ?? []).map((s, i) =>
        i === index ? { ...s, ...patch } : s,
      ),
    });
  return (
    <div className="space-y-5 text-zinc-100">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold flex items-center gap-3">
            {path === "notebooks" ? (
              <BookOpen className="text-emerald-400" />
            ) : path === "workflows" ? (
              <Workflow className="text-emerald-400" />
            ) : path === "databases" ? (
              <Database className="text-emerald-400" />
            ) : path === "vulnerabilidades" ? (
              <ShieldCheck className="text-emerald-400" />
            ) : path === "mcp" ? (
              <Terminal className="text-emerald-400" />
            ) : (
              <Globe2 className="text-emerald-400" />
            )}
            {labels[path]}
          </h1>
          <p className="text-zinc-400 text-sm mt-2">
            {path === "notebooks"
              ? "Investigue sua telemetria com consultas VQL e anotações."
              : path === "workflows"
                ? "Organize consultas, condições e verificações. Execute agora ou por intervalo."
                : path === "databases"
                  ? "Inventário observado e verificações de PostgreSQL em modo de leitura."
                  : path === "vulnerabilidades"
                    ? "Avalie versões de pacotes com os avisos públicos do OSV."
                    : path === "frontends"
                      ? "Serviços que responderam com HTML nas portas observadas pelo agente."
                      : "Conecte seu cliente de IA para consultar e configurar a Vetra."}
          </p>
        </div>
        {kind && (
          <button className={button} disabled={demo} onClick={create}>
            <Plus size={16} />
            Novo
          </button>
        )}
      </header>
      {demo && (
        <p className={box}>
          Conecte um servidor para usar estas funções com dados reais.
        </p>
      )}
      {(message || data.error || inventory.error) && (
        <p
          role="status"
          className="border border-emerald-900 bg-emerald-950 p-3 text-sm"
        >
          {message || data.error?.message || inventory.error?.message}
        </p>
      )}
      {path === "mcp" && (
        <section className={box}>
          <h2 className="font-semibold">Configuração por comandos de IA</h2>
          <p>
            Endpoint: <code>{window.location.origin}/mcp</code>. Use o token
            administrativo no cabeçalho <code>X-API-Key</code> do cliente MCP.
          </p>
          <p className="text-sm text-zinc-400">
            O cliente precisa oferecer suporte ao transporte HTTP de requisição
            e resposta. Não há um modelo de IA embutido, OAuth ou permissões por
            ferramenta.
          </p>
          <Table
            rows={[
              { ferramenta: "run_query", ação: "Consulta VQL" },
              {
                ferramenta: "configure_workspace",
                ação: "Cria e edita notebooks, workflows, perfis de banco e software",
              },
              {
                ferramenta: "configure_rum",
                ação: "Configura aplicações RUM e privacidade",
              },
              {
                ferramenta: "configure_host",
                ação: "Renomeia e organiza hosts",
              },
              {
                ferramenta: "run_operation",
                ação: "Executa workflows, avaliação de pacotes e verificação de banco",
              },
            ]}
          />
          <p>
            As ferramentas de escrita exibem uma prévia com{" "}
            <code>apply: false</code>. O cliente envia <code>apply: true</code>{" "}
            para aplicar.
          </p>
          <pre className="overflow-auto text-xs bg-black p-4">
            {JSON.stringify(
              {
                jsonrpc: "2.0",
                id: 1,
                method: "tools/call",
                params: {
                  name: "run_query",
                  arguments: { query: "fetch hosts | fields name, os" },
                },
              },
              null,
              2,
            )}
          </pre>
        </section>
      )}
      {path === "frontends" && (
        <section className={box}>
          <details>
            <summary className="cursor-pointer text-sm text-zinc-400">
              Detalhes da descoberta
            </summary>
            <Table rows={inventory.data?.frontends ?? []} />
          </details>
          <div className="grid gap-3 md:grid-cols-2">
            {inventory.data?.frontends?.map((f) => {
              const rum = (f["rum"] ?? {}) as Record<string, unknown>;
              return (
                <section
                  key={String(f["id"])}
                  className="space-y-3 border border-emerald-900 bg-black p-4"
                >
                  <h3 className="font-semibold">
                    {String(f["service"])} · {String(f["host_name"])}
                  </h3>
                  <p className="text-xs text-zinc-400">
                    {String(rum["sessions"] ?? 0)} sessões ·{" "}
                    {String(rum["pages"] ?? 0)} acessos ·{" "}
                    {String(rum["replay_sessions"] ?? 0)} sessões com replay
                  </p>
                  {rum["configured"] ? (
                    <div className="flex gap-2 flex-wrap">
                      {!!rum["test_url"] && (
                        <a
                          className={button}
                          href={String(rum["test_url"])}
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          Visitar aplicação
                        </a>
                      )}
                      <a
                        className={button}
                        href={`/rum?app_id=${encodeURIComponent(String(f["id"]))}`}
                      >
                        Ver acessos
                      </a>
                      <a
                        className={button}
                        href={`/session-replay?app_id=${encodeURIComponent(String(f["id"]))}`}
                      >
                        Ver Session Replays
                      </a>
                      <a
                        className={button}
                        href={`/experience-vitals?app_id=${encodeURIComponent(String(f["id"]))}`}
                      >
                        Experience Vitals
                      </a>
                    </div>
                  ) : (
                    <p className="text-xs text-zinc-400">
                      Configure RUM abaixo e abra a aplicação pelo caminho
                      instrumentado para gerar uma sessão real.
                    </p>
                  )}
                  {!!rum["configured"] && !rum["replay_enabled"] && (
                    <p className="text-xs text-amber-200">
                      Replay desativado. Ative na configuração RUM, revise a
                      amostragem e autorize no navegador de teste.
                    </p>
                  )}
                </section>
              );
            })}
          </div>

          <div className="flex gap-3 flex-wrap">
            {inventory.data?.frontends
              ?.filter(
                (f) =>
                  !(f["rum"] as Record<string, unknown> | undefined)?.[
                    "configured"
                  ],
              )
              .map((f) => (
                <button
                  key={String(f["id"])}
                  className={button}
                  onClick={() => {
                    setCandidate(f);
                    setOrigin("");
                  }}
                >
                  Configurar RUM · {String(f["service"])}:{String(f["port"])}
                </button>
              ))}
          </div>
          <p className="text-sm text-zinc-400">
            A descoberta não inicia a captura do navegador. Informe a origem
            pública e instale o SDK ou configure o proxy de injeção. O
            consentimento e o mascaramento continuam obrigatórios conforme as
            regras da aplicação.
          </p>
          {candidate && (
            <div className="space-y-3 border-t border-zinc-800 pt-4">
              <label className="block text-sm">
                Origem pública da aplicação
                <input
                  className={input}
                  placeholder="https://app.empresa.com"
                  value={origin}
                  onChange={(e) => setOrigin(e.target.value)}
                />
              </label>
              <button
                className={button}
                disabled={busy || !origin}
                onClick={() =>
                  void perform(async () => {
                    const u = new URL(origin);
                    if (
                      !["https:", "http:"].includes(u.protocol) ||
                      u.origin !== origin
                    )
                      throw new Error(
                        "Informe apenas a origem, sem caminho ou barra final.",
                      );
                    const existing = await request<{
                      applications: { id: string }[];
                    }>("/api/v1/rum/apps");
                    if (
                      existing.applications.some(
                        (a) => a.id === String(candidate["id"]),
                      )
                    )
                      throw new Error(
                        "Aplicação já configurada. Edite as opções no menu RUM.",
                      );
                    await request("/api/v1/rum/apps", {
                      method: "POST",
                      body: JSON.stringify({
                        id: String(candidate["id"]),
                        name: String(candidate["service"]),
                        enabled: true,
                        origins: [origin],
                        masking: "all",
                        consent_banner: true,
                        sample_rate: 1,
                        replay_enabled: false,
                        replay_sample_rate: 0,
                        apdex_t_ms: 500,
                        require_consent: true,
                        block_selectors: [],
                      }),
                    });
                  }, "Aplicação criada. Abra RUM para revisar privacidade e obter a instalação.")
                }
              >
                Criar aplicação RUM
              </button>
            </div>
          )}
        </section>
      )}
      {path === "databases" && (
        <section className={box}>
          <h2 className="font-semibold">Bancos observados</h2>
          <Table rows={inventory.data?.databases ?? []} />
          <p className="text-sm text-zinc-400">
            Processos e atributos de spans indicam presença. A conexão SQL só é
            verificada nos perfis PostgreSQL configurados abaixo.
          </p>
        </section>
      )}
      {path === "vulnerabilidades" && (
        <section className={box}>
          <h2 className="font-semibold">Importar inventário</h2>
          <label className="block text-sm">
            CycloneDX JSON ou package-lock v2/v3
            <input
              className={input}
              type="file"
              accept=".json,application/json"
              disabled={busy || demo}
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file)
                  void perform(async () => {
                    if (file.size > 4 * 1024 * 1024)
                      throw new Error("Arquivo maior que 4 MiB.");
                    await request("/api/v1/software/import", {
                      method: "POST",
                      body: JSON.stringify({
                        name: file.name,
                        document: JSON.parse(await file.text()),
                      }),
                    });
                  }, "Inventário importado. Selecione o perfil e execute a avaliação.");
              }}
            />
          </label>
          <div className="flex gap-2">
            <input
              className={input}
              aria-label="ID do host"
              placeholder="ID do host para importar os pacotes descobertos"
              value={hostID}
              onChange={(e) => setHostID(e.target.value)}
            />
            <button
              className={button}
              disabled={busy || demo || !hostID}
              onClick={() =>
                void perform(
                  () =>
                    request(
                      `/api/v1/software/from-host/${encodeURIComponent(hostID)}`,
                      { method: "POST" },
                    ),
                  "Pacotes do agente importados.",
                )
              }
            >
              Importar do agente
            </button>
          </div>
          <p className="text-sm text-zinc-400">
            A consulta envia nomes e versões dos pacotes ao OSV. Não avalia
            exploração em execução ou vulnerabilidades do código como o
            Dynatrace. Um resultado parcial não comprova ausência de
            vulnerabilidades.
          </p>
        </section>
      )}
      {kind && (
        <div className="grid gap-5 xl:grid-cols-[280px_1fr]">
          <aside className={box}>
            <h2 className="font-semibold">
              {path === "databases" ? "Conexões configuradas" : "Salvos"}
            </h2>
            {objects
              .filter((o) => o.kind === kind)
              .map((o) => (
                <button
                  className={`block w-full text-left px-3 py-3 border ${editing?.id === o.id ? "bg-emerald-950 border-emerald-700" : "bg-black border-zinc-800"}`}
                  key={o.id}
                  onClick={() => {
                    setEditing(structuredClone(o));
                    setResults({});
                  }}
                >
                  {o.name}
                  <span className="block text-xs text-zinc-500 mt-1">
                    {runs.find((r) => r.object_id === o.id)?.status ??
                      "Sem execução"}
                  </span>
                </button>
              ))}
            {!objects.some((o) => o.kind === kind) && (
              <p className="text-sm text-zinc-500">
                Nenhuma configuração salva.
              </p>
            )}
          </aside>
          <main className="space-y-5">
            {editing ? (
              <section className={box}>
                <label className="block text-sm">
                  Nome
                  <input
                    className={input}
                    value={editing.name}
                    onChange={(e) =>
                      setEditing({ ...editing, name: e.target.value })
                    }
                  />
                </label>
                {editing.kind === "notebook" && (
                  <>
                    <p className="text-xs text-zinc-400">
                      VQL 1 · fetch → filter → fields → sort → summarize →
                      limit. Fontes: hosts, processes, services, spans, logs,
                      metrics, rum, frontends, databases, vulnerabilities,
                      events. Janela: 60 minutos. Limite de saída: 1.000 linhas.
                    </p>
                    {(editing.cells ?? []).map((c, i) => (
                      <div
                        key={c.id}
                        className="space-y-3 border border-zinc-800 bg-black p-4"
                      >
                        <div className="flex gap-2">
                          <select
                            className={input}
                            aria-label={`Tipo da célula ${i + 1}`}
                            value={c.kind}
                            onChange={(e) =>
                              setEditing({
                                ...editing,
                                cells: (editing.cells ?? []).map((x) =>
                                  x.id === c.id
                                    ? { ...x, kind: e.target.value }
                                    : x,
                                ),
                              })
                            }
                          >
                            <option value="query">Consulta VQL</option>
                            <option value="markdown">Anotação</option>
                          </select>
                          <button
                            className={button}
                            aria-label={`Remover célula ${i + 1}`}
                            onClick={() =>
                              setEditing({
                                ...editing,
                                cells: (editing.cells ?? []).filter(
                                  (x) => x.id !== c.id,
                                ),
                              })
                            }
                          >
                            <Trash2 size={15} />
                          </button>
                        </div>
                        <textarea
                          aria-label={`Conteúdo da célula ${i + 1}`}
                          className={`${input} font-mono min-h-24`}
                          value={c.content}
                          onChange={(e) =>
                            setEditing({
                              ...editing,
                              cells: (editing.cells ?? []).map((x) =>
                                x.id === c.id
                                  ? { ...x, content: e.target.value }
                                  : x,
                              ),
                            })
                          }
                        />
                        {c.kind === "query" ? (
                          <button
                            className={button}
                            disabled={busy || demo}
                            onClick={() =>
                              void perform(async () => {
                                const result = await request<{ rows: Row[] }>(
                                  "/api/v1/vql",
                                  {
                                    method: "POST",
                                    body: JSON.stringify({
                                      query: c.content,
                                      minutes: 60,
                                    }),
                                  },
                                );
                                setResults((r) => ({
                                  ...r,
                                  [c.id]: result.rows,
                                }));
                              }, "Consulta concluída.")
                            }
                          >
                            <Play size={15} />
                            Executar célula
                          </button>
                        ) : (
                          <p className="whitespace-pre-wrap text-zinc-300">
                            {c.content}
                          </p>
                        )}
                        {results[c.id] && <Table rows={results[c.id]!} />}
                      </div>
                    ))}
                    <button
                      className={button}
                      onClick={() =>
                        setEditing({
                          ...editing,
                          cells: [
                            ...(editing.cells ?? []),
                            {
                              id: id(),
                              kind: "query",
                              content: "fetch services | limit 100",
                            },
                          ],
                        })
                      }
                    >
                      <Plus size={15} />
                      Adicionar célula
                    </button>
                  </>
                )}
                {editing.kind === "database" && (
                  <>
                    <p>PostgreSQL · Transação somente leitura</p>
                    <label className="block text-sm">
                      Variável de ambiente com a conexão
                      <input
                        className={input}
                        value={editing.credential_env ?? ""}
                        onChange={(e) =>
                          setEditing({
                            ...editing,
                            credential_env: e.target.value,
                          })
                        }
                        placeholder="OBSERVA_DB_PRINCIPAL_DSN"
                      />
                    </label>
                    <p className="text-sm text-zinc-400">
                      Defina essa variável no servidor e repasse-a ao container.
                      O DSN e a senha não são gravados neste formulário. São
                      coletados tamanho, sessões, transações, deadlocks e cache.
                      Destinos privados exigem
                      OBSERVA_ALLOW_PRIVATE_TARGETS=true.
                    </p>
                  </>
                )}
                {editing.kind === "software" && (
                  <>
                    {(editing.packages ?? []).map((p, i) => (
                      <div className="grid gap-2 md:grid-cols-3" key={i}>
                        <input
                          aria-label={`Pacote ${i + 1}`}
                          className={input}
                          placeholder="Nome do pacote"
                          value={p.name}
                          onChange={(e) =>
                            setEditing({
                              ...editing,
                              packages: (editing.packages ?? []).map((x, j) =>
                                i === j ? { ...x, name: e.target.value } : x,
                              ),
                            })
                          }
                        />
                        <input
                          aria-label={`Versão ${i + 1}`}
                          className={input}
                          placeholder="Versão exata"
                          value={p.version}
                          onChange={(e) =>
                            setEditing({
                              ...editing,
                              packages: (editing.packages ?? []).map((x, j) =>
                                i === j ? { ...x, version: e.target.value } : x,
                              ),
                            })
                          }
                        />
                        <select
                          className={input}
                          aria-label={`Ecossistema ${i + 1}`}
                          value={p.ecosystem}
                          onChange={(e) =>
                            setEditing({
                              ...editing,
                              packages: (editing.packages ?? []).map((x, j) =>
                                i === j
                                  ? { ...x, ecosystem: e.target.value }
                                  : x,
                              ),
                            })
                          }
                        >
                          {[
                            "npm",
                            "PyPI",
                            "Go",
                            "Maven",
                            "NuGet",
                            "Packagist",
                            "RubyGems",
                            "crates.io",
                          ].map((x) => (
                            <option key={x}>{x}</option>
                          ))}
                        </select>
                      </div>
                    ))}
                    <button
                      className={button}
                      onClick={() =>
                        setEditing({
                          ...editing,
                          packages: [
                            ...(editing.packages ?? []),
                            { name: "", version: "", ecosystem: "npm" },
                          ],
                        })
                      }
                    >
                      Adicionar pacote
                    </button>
                  </>
                )}
                {editing.kind === "workflow" && (
                  <>
                    <label className="flex gap-2 text-sm">
                      <input
                        type="checkbox"
                        checked={editing.enabled}
                        onChange={(e) =>
                          setEditing({ ...editing, enabled: e.target.checked })
                        }
                      />
                      Ativar agendamento
                    </label>
                    <label className="block text-sm">
                      Intervalo em segundos (0 para execução manual)
                      <input
                        type="number"
                        min="0"
                        max="86400"
                        className={input}
                        value={editing.interval_seconds ?? 0}
                        onChange={(e) =>
                          setEditing({
                            ...editing,
                            interval_seconds: Number(e.target.value),
                          })
                        }
                      />
                    </label>
                    {(editing.steps ?? []).map((s, i) => (
                      <div
                        className="border border-zinc-800 bg-black p-4 space-y-3"
                        key={s.id}
                      >
                        <div className="flex gap-2">
                          <input
                            aria-label={`Nome da etapa ${i + 1}`}
                            className={input}
                            value={s.name}
                            onChange={(e) =>
                              patchStep(i, { name: e.target.value })
                            }
                          />
                          <select
                            aria-label={`Tipo da etapa ${i + 1}`}
                            className={input}
                            value={s.kind}
                            onChange={(e) =>
                              patchStep(i, { kind: e.target.value })
                            }
                          >
                            <option value="query">Consulta VQL</option>
                            <option value="condition">
                              Condição: mínimo de linhas
                            </option>
                            <option value="database_check">
                              Verificar banco
                            </option>
                            <option value="vulnerability_scan">
                              Avaliar vulnerabilidades
                            </option>
                          </select>
                          <button
                            className={button}
                            aria-label={`Remover etapa ${i + 1}`}
                            onClick={() =>
                              setEditing({
                                ...editing,
                                steps: (editing.steps ?? []).filter(
                                  (x) => x.id !== s.id,
                                ),
                              })
                            }
                          >
                            <Trash2 size={15} />
                          </button>
                        </div>
                        {s.kind === "query" ? (
                          <textarea
                            aria-label={`Consulta da etapa ${i + 1}`}
                            className={`${input} font-mono`}
                            value={s.query ?? ""}
                            onChange={(e) =>
                              patchStep(i, { query: e.target.value })
                            }
                          />
                        ) : s.kind === "condition" ? (
                          <label className="block text-sm">
                            Continuar quando a consulta anterior retornar ao
                            menos
                            <input
                              type="number"
                              min="0"
                              max="15000"
                              className={input}
                              value={s.min_rows ?? 0}
                              onChange={(e) =>
                                patchStep(i, {
                                  min_rows: Number(e.target.value),
                                })
                              }
                            />
                          </label>
                        ) : (
                          <select
                            aria-label={`Perfil da etapa ${i + 1}`}
                            className={input}
                            value={s.target_id ?? ""}
                            onChange={(e) =>
                              patchStep(i, { target_id: e.target.value })
                            }
                          >
                            <option value="">Selecione um perfil</option>
                            {objects
                              .filter(
                                (o) =>
                                  o.kind ===
                                  (s.kind === "database_check"
                                    ? "database"
                                    : "software"),
                              )
                              .map((o) => (
                                <option key={o.id} value={o.id}>
                                  {o.name}
                                </option>
                              ))}
                          </select>
                        )}
                      </div>
                    ))}
                    <button
                      className={button}
                      onClick={() =>
                        setEditing({
                          ...editing,
                          steps: [
                            ...(editing.steps ?? []),
                            {
                              id: id(),
                              name: "Nova etapa",
                              kind: "query",
                              query: "fetch events | limit 100",
                            },
                          ],
                        })
                      }
                    >
                      Adicionar etapa
                    </button>
                  </>
                )}
                <div className="flex gap-2 flex-wrap">
                  <button
                    className={button}
                    disabled={busy || demo || !editing.name}
                    onClick={() => void save()}
                  >
                    <Save size={15} />
                    Salvar
                  </button>
                  {editing.kind !== "notebook" && (
                    <button
                      className={button}
                      disabled={
                        busy ||
                        demo ||
                        !objects.some((o) => o.id === editing.id)
                      }
                      onClick={() => void run(editing)}
                    >
                      <Play size={15} />
                      Executar versão salva
                    </button>
                  )}
                  <button
                    className={button}
                    disabled={
                      busy || demo || !objects.some((o) => o.id === editing.id)
                    }
                    onClick={() =>
                      void perform(async () => {
                        await request(`/api/v1/workspace/${editing.id}`, {
                          method: "DELETE",
                        });
                        setEditing(null);
                      }, "Configuração excluída.")
                    }
                  >
                    <Trash2 size={15} />
                    Excluir
                  </button>
                </div>
              </section>
            ) : (
              <section className={box}>
                <p>Selecione uma configuração ou crie uma nova.</p>
              </section>
            )}
            {editing &&
              runs
                .filter((r) => r.object_id === editing.id)
                .slice(0, 10)
                .map((r) => (
                  <section className={box} key={r.id}>
                    <h3 className="font-medium">
                      {r.status === "running"
                        ? "Em execução"
                        : r.status === "success"
                          ? "Concluído"
                          : "Falhou"}{" "}
                      · {new Date(r.started_at).toLocaleString("pt-BR")}
                    </h3>
                    {r.error && <p className="text-red-300">{r.error}</p>}
                    {r.result["query_complete"] === false && (
                      <p className="text-amber-300">
                        Avaliação parcial: nem todas as páginas foram
                        consultadas.
                      </p>
                    )}
                    {r.result["details_complete"] === false && (
                      <p className="text-amber-300">
                        Detalhes de alguns avisos indisponíveis.
                      </p>
                    )}
                    {Array.isArray(r.result["findings"]) && (
                      <p className="text-sm text-zinc-300">
                        OSV · {String(r.result["packages_scanned"] ?? 0)}{" "}
                        pacotes avaliados · {String(r.result["matches"] ?? 0)}{" "}
                        avisos retornados. Correspondência de nome e versão;
                        exploração em execução não avaliada.
                      </p>
                    )}
                    {Array.isArray(r.result["findings"]) ? (
                      <Table rows={r.result["findings"] as Row[]} />
                    ) : Array.isArray(r.result["steps"]) ? (
                      <Table rows={r.result["steps"] as Row[]} />
                    ) : (
                      <Table rows={[r.result]} />
                    )}
                    <details>
                      <summary className="cursor-pointer text-xs text-emerald-400">
                        Resultado completo
                      </summary>
                      <pre className="text-xs overflow-auto max-h-64 p-3 bg-black">
                        {JSON.stringify(r.result, null, 2)}
                      </pre>
                    </details>
                  </section>
                ))}
          </main>
        </div>
      )}
    </div>
  );
}
