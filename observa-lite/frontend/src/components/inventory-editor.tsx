import { useState } from "react";
import { request, type Host, type ProcessInfo } from "@/lib/api";
import { Button } from "./ui/button";
import { Input } from "./ui/input";

export function InventoryEditor({
  host,
  process,
  groups,
  onSaved,
}: {
  host: Host;
  process?: ProcessInfo;
  groups: string[];
  onSaved?: () => void;
}) {
  const [name, setName] = useState(
    process ? process.display_name || process.name : host.name,
  );
  const [group, setGroup] = useState(host.host_group || "");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  async function save(reset = false) {
    setBusy(true);
    setMessage("");
    try {
      await request(`/api/v1/hosts/${encodeURIComponent(host.id)}`, {
        method: "PATCH",
        body: JSON.stringify(
          reset
            ? { reset: true }
            : process
              ? {
                  pid: process.pid,
                  process_name: name,
                  original_name: process.name,
                  original_service: process.service,
                }
              : { name, host_group: group },
        ),
      });
      setMessage("Alterações salvas.");
      onSaved?.();
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : "Não foi possível salvar.",
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="space-y-3 rounded-sm border border-border bg-surface p-4">
      <h3 className="text-sm font-semibold">
        {process ? "Editar processo" : "Editar host"}
      </h3>
      <label className="block space-y-1 text-xs">
        Nome de exibição
        <Input
          value={name}
          maxLength={128}
          onChange={(e) => setName(e.target.value)}
        />
      </label>
      {!process && (
        <label className="block space-y-1 text-xs">
          Grupo de hosts
          <Input
            list="inventory-host-groups"
            value={group}
            maxLength={100}
            onChange={(e) => setGroup(e.target.value)}
            placeholder="Selecione ou crie um grupo"
          />
          <datalist id="inventory-host-groups">
            {[...new Set(groups)].map((g) => (
              <option key={g} value={g} />
            ))}
          </datalist>
        </label>
      )}
      <p className="text-xs text-muted-foreground">
        {process
          ? "Altera o nome exibido. O executável e a identificação coletada são preservados."
          : "Um nome de grupo novo cria o grupo ao salvar. As alterações permanecem após novas coletas."}
      </p>
      <div className="flex gap-2">
        <Button disabled={busy} onClick={() => void save()}>
          Salvar alterações
        </Button>
        {!process && (
          <Button
            variant="outline"
            disabled={busy}
            onClick={() => void save(true)}
          >
            Usar dados do agente
          </Button>
        )}
      </div>
      <p role="status" className="text-xs">
        {message}
      </p>
    </section>
  );
}
