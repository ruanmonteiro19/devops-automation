import { useState, type ReactNode, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { request, readToken, saveToken } from "@/lib/api";
import { ShieldCheck, Users, KeyRound, LogOut, Save } from "lucide-react";
type User = {
  username: string;
  name: string;
  role: string;
  disabled: boolean;
  must_change_password: boolean;
};
const input =
  "w-full rounded-sm border border-emerald-900 bg-black px-3 py-2 text-sm";
const button =
  "inline-flex items-center gap-2 border border-emerald-800 bg-emerald-950 px-4 py-2 text-sm text-emerald-100 disabled:opacity-40";
function PasswordForm({ done }: { done: () => void }) {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await request("/api/v1/auth/password", {
        method: "POST",
        body: JSON.stringify({ current_password: current, new_password: next }),
      });
      window.sessionStorage.removeItem("vetra-csrf");
      done();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Falha na troca");
    } finally {
      setBusy(false);
    }
  };
  return (
    <form onSubmit={(e) => void submit(e)} className="space-y-4">
      <h2 className="flex items-center gap-2 font-semibold">
        <KeyRound className="text-emerald-400" />
        Alterar senha
      </h2>
      <label className="block text-sm">
        Senha atual
        <input
          className={input}
          type="password"
          autoComplete="current-password"
          required
          value={current}
          onChange={(e) => setCurrent(e.target.value)}
        />
      </label>
      <label className="block text-sm">
        Nova senha
        <input
          className={input}
          type="password"
          autoComplete="new-password"
          required
          minLength={8}
          maxLength={72}
          value={next}
          onChange={(e) => setNext(e.target.value)}
        />
      </label>
      <p className="text-xs text-zinc-400">
        Use pelo menos 8 caracteres. Após a troca, entre novamente.
      </p>
      <button className={button} disabled={busy}>
        Salvar nova senha
      </button>
      {message && <p role="alert">{message}</p>}
    </form>
  );
}
export function AuthGate({ children }: { children: ReactNode }) {
  const qc = useQueryClient();
  const [user, setUser] = useState("admin");
  const [password, setPassword] = useState("");
  const [token, setToken] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const session = useQuery({
    queryKey: ["auth-session"],
    queryFn: () =>
      request<{ user: User; csrf: string }>("/api/v1/auth/session"),
    enabled: !readToken(),
    retry: false,
    staleTime: 30000,
    refetchInterval: 60000,
  });
  const refresh = async () => {
    qc.setQueryData(["auth-session"], null);
    await qc.invalidateQueries({ queryKey: ["auth-session"] });
    await qc.invalidateQueries({ queryKey: ["snapshot"] });
  };
  if (readToken()) return children;
  if (session.isLoading)
    return (
      <main className="grid min-h-screen place-items-center bg-black text-emerald-200">
        Conectando à Vetra…
      </main>
    );
  if (
    !session.error &&
    session.data?.user &&
    !session.data.user.must_change_password
  )
    return children;
  const login = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await request("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ username: user, password }),
      });
      setPassword("");
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível entrar");
    } finally {
      setBusy(false);
    }
  };
  return (
    <main className="grid min-h-screen place-items-center bg-black p-6 text-zinc-100">
      <section className="w-full max-w-md space-y-6 border border-emerald-900 bg-zinc-950 p-8">
        <h1 className="flex items-center gap-3 text-2xl font-semibold">
          <ShieldCheck className="text-emerald-400" />
          Vetra Observability
        </h1>
        {!session.error && session.data?.user.must_change_password ? (
          <>
            <p className="text-sm text-zinc-400">
              Defina sua senha pessoal para concluir o primeiro acesso.
            </p>
            <PasswordForm done={() => void refresh()} />
          </>
        ) : (
          <>
            <form onSubmit={(e) => void login(e)} className="space-y-4">
              <label className="block text-sm">
                Usuário
                <input
                  className={input}
                  autoComplete="username"
                  required
                  value={user}
                  onChange={(e) => setUser(e.target.value)}
                />
              </label>
              <label className="block text-sm">
                Senha
                <input
                  className={input}
                  type="password"
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </label>
              <button className={button} disabled={busy}>
                Entrar
              </button>
              {error && <p role="alert">{error}</p>}
            </form>
            <details className="border-t border-zinc-800 pt-4 text-sm">
              <summary className="cursor-pointer text-zinc-400">
                Acesso por token administrativo
              </summary>
              <input
                className={`${input} mt-3`}
                type="password"
                aria-label="Token administrativo"
                value={token}
                onChange={(e) => setToken(e.target.value)}
              />
              <button
                className={`${button} mt-2`}
                onClick={() => {
                  saveToken(token.trim());
                  window.location.reload();
                }}
              >
                Conectar com token
              </button>
            </details>
          </>
        )}
      </section>
    </main>
  );
}
export function UsersPage() {
  const qc = useQueryClient();
  const session = useQuery({
    queryKey: ["auth-session"],
    queryFn: () => request<{ user: User }>("/api/v1/auth/session"),
  });
  const admin = !!readToken() || session.data?.user.role === "admin";
  const data = useQuery({
    queryKey: ["users"],
    queryFn: () => request<{ users: User[] }>("/api/v1/users"),
    enabled: admin,
  });
  const [draft, setDraft] = useState({
    username: "",
    name: "",
    password: "",
    role: "viewer",
    disabled: false,
  });
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const save = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await request("/api/v1/users", {
        method: "POST",
        body: JSON.stringify(draft),
      });
      setDraft({
        username: "",
        name: "",
        password: "",
        role: "viewer",
        disabled: false,
      });
      setMessage("Usuário salvo.");
      await qc.invalidateQueries({ queryKey: ["users"] });
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Falha ao salvar");
    } finally {
      setBusy(false);
    }
  };
  const logout = async () => {
    if (!readToken()) await request("/api/v1/auth/logout", { method: "POST" });
    saveToken("");
    window.sessionStorage.removeItem("vetra-csrf");
    window.location.reload();
  };
  return (
    <div className="space-y-6">
      <header className="flex justify-between">
        <h1 className="flex items-center gap-3 text-2xl font-semibold">
          <Users className="text-emerald-400" />
          Usuários e acesso
        </h1>
        <button className={button} onClick={() => void logout()}>
          <LogOut size={16} />
          Sair
        </button>
      </header>
      {(message || data.error) && (
        <p
          role="status"
          className="border border-emerald-900 bg-emerald-950 p-3"
        >
          {message || data.error?.message}
        </p>
      )}
      {admin && (
        <div className="grid gap-6 lg:grid-cols-2">
          <section className="space-y-3 border border-zinc-800 bg-zinc-950 p-5">
            <h2 className="font-semibold">Contas cadastradas</h2>
            {data.data?.users.map((u) => (
              <button
                key={u.username}
                className="block w-full border border-zinc-800 bg-black p-3 text-left text-sm"
                onClick={() =>
                  setDraft({
                    username: u.username,
                    name: u.name,
                    password: "",
                    role: u.role,
                    disabled: u.disabled,
                  })
                }
              >
                {u.name || u.username}
                <span className="block text-xs text-zinc-400">
                  {u.username} ·{" "}
                  {u.role === "admin" ? "Administrador" : "Leitura"} ·{" "}
                  {u.disabled ? "Desativado" : "Ativo"}
                </span>
              </button>
            ))}
          </section>
          <form
            className="space-y-4 border border-zinc-800 bg-zinc-950 p-5"
            onSubmit={(e) => void save(e)}
          >
            <h2 className="font-semibold">Criar ou editar usuário</h2>
            <label className="block text-sm">
              Nome
              <input
                className={input}
                value={draft.name}
                onChange={(e) => setDraft({ ...draft, name: e.target.value })}
              />
            </label>
            <label className="block text-sm">
              Identificador do usuário
              <input
                className={input}
                required
                pattern="[a-zA-Z0-9_.-]{1,64}"
                value={draft.username}
                onChange={(e) =>
                  setDraft({ ...draft, username: e.target.value })
                }
              />
            </label>
            <label className="block text-sm">
              Senha inicial ou redefinição
              <input
                className={input}
                type="password"
                autoComplete="new-password"
                minLength={8}
                value={draft.password}
                onChange={(e) =>
                  setDraft({ ...draft, password: e.target.value })
                }
              />
            </label>
            <p className="text-xs text-zinc-400">
              Ao editar, deixe em branco para manter a senha. Uma senha
              redefinida precisa ser trocada no próximo acesso.
            </p>
            <label className="block text-sm">
              Perfil
              <select
                className={input}
                value={draft.role}
                onChange={(e) => setDraft({ ...draft, role: e.target.value })}
              >
                <option value="viewer">Leitura</option>
                <option value="admin">Administrador</option>
              </select>
            </label>
            <label className="flex gap-2 text-sm">
              <input
                type="checkbox"
                checked={draft.disabled}
                onChange={(e) =>
                  setDraft({ ...draft, disabled: e.target.checked })
                }
              />
              Desativar conta
            </label>
            <button className={button} disabled={busy}>
              <Save size={16} />
              Salvar usuário
            </button>
          </form>
        </div>
      )}
      {!readToken() && (
        <section className="max-w-lg border border-zinc-800 bg-zinc-950 p-5">
          <PasswordForm done={() => window.location.reload()} />
        </section>
      )}
      {readToken() && (
        <p className="text-sm text-zinc-400">
          Você está conectado por token. Entre com usuário e senha para alterar
          a senha da sua conta.
        </p>
      )}
    </div>
  );
}
