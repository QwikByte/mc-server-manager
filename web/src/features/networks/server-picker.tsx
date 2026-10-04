import { PuzzlePieceIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { Chip } from "@/components/chip"
import { Checkbox } from "@/components/ui/checkbox"
import type { NodeServer } from "@/features/servers/api"
import { serverType } from "@/features/servers/server-types"
import { cn } from "@/lib/utils"
import type { Forwarding, ServerRef } from "./api"
import { canJoin, forwardingMod, key, refOf } from "./servers"

/** Why a server can't join a network with the forwarding, if it can't. */
function reason(type: string, forwarding: Forwarding) {
  if (canJoin(type, forwarding)) return undefined
  if (type === "fabric") return t("FabricProxy-Lite only supports Velocity's modern forwarding.")
  return t("Vanilla servers can't verify the players a proxy forwards.")
}

/** Picks the servers that join a network, in the order they are picked. */
export function ServerPicker({
  servers,
  forwarding,
  selected,
  onChange,
}: {
  servers: NodeServer[]
  forwarding: Forwarding
  selected: ServerRef[]
  onChange: (refs: ServerRef[]) => void
}) {
  if (servers.length === 0)
    return <p className="rounded-lg bg-muted/60 p-4 text-sm text-muted-foreground">{t("No game server is free to join. Create a Paper, Purpur, Fabric, Forge or NeoForge server first.")}</p>
  const order = selected.map(key)
  const sorted = [...servers].sort((a, b) => Number(!canJoin(a.type, forwarding)) - Number(!canJoin(b.type, forwarding)) || a.name.localeCompare(b.name))

  return (
    <ul className="max-h-80 divide-y overflow-y-auto rounded-lg ring-1 ring-border">
      {sorted.map((s) => {
        const k = key(refOf(s))
        const position = order.indexOf(k)
        const why = reason(s.type, forwarding)
        const mod = forwardingMod(s.type)
        return (
          <li key={k}>
            <label className={cn("flex cursor-pointer items-center gap-3 px-3 py-2.5 hover:bg-muted/50", why && "cursor-not-allowed opacity-60")}>
              <Checkbox
                checked={position >= 0}
                disabled={!!why}
                onCheckedChange={(on) => onChange(on ? [...selected, refOf(s)] : selected.filter((r) => key(r) !== k))}
              />
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-2 text-sm font-medium">
                  {s.name}
                  <span className="font-normal text-muted-foreground">{t("{{node}} · port {{port}}", { node: s.nodeName, port: s.port })}</span>
                </span>
                {why && <span className="block text-xs text-muted-foreground">{why}</span>}
              </span>
              {mod && !why && (
                <Chip icon={PuzzlePieceIcon} className="max-sm:hidden">
                  {t("+ {{mod}}", { mod })}
                </Chip>
              )}
              <Chip>{serverType(s.type).label}</Chip>
              <span aria-hidden className={cn("grid size-6 place-items-center rounded-md text-xs font-bold tabular-nums", position >= 0 ? "bg-primary text-primary-foreground" : "invisible")}>
                {position + 1}
              </span>
            </label>
          </li>
        )
      })}
    </ul>
  )
}
