import { MagnifyingGlassIcon, PuzzlePieceIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { useState } from "react"
import { Chip } from "@/components/chip"
import { Button } from "@/components/ui/button"
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
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
  const [search, setSearch] = useState("")
  if (servers.length === 0)
    return <p className="rounded-lg bg-muted/60 p-4 text-sm text-muted-foreground">{t("No game server is free to join. Create a Paper, Purpur, Fabric, Forge or NeoForge server first.")}</p>
  const order = selected.map(key)
  const words = search.toLowerCase().split(/\s+/).filter(Boolean)
  const sorted = [...servers]
    .filter((s) => words.every((w) => [s.name, s.nodeName, serverType(s.type).label, ...s.tags.map((tag) => `#${tag}`)].join(" ").toLowerCase().includes(w)))
    .sort((a, b) => Number(!canJoin(a.type, forwarding)) - Number(!canJoin(b.type, forwarding)) || a.name.localeCompare(b.name, undefined, { numeric: true }))
  const joinable = sorted.filter((s) => canJoin(s.type, forwarding))
  const all = joinable.length > 0 && joinable.every((s) => order.includes(key(refOf(s))))

  return (
    <div className="grid gap-2">
      {servers.length > 8 && (
        <div className="flex items-center gap-2">
          <InputGroup>
            <InputGroupAddon>
              <MagnifyingGlassIcon />
            </InputGroupAddon>
            <InputGroupInput type="search" placeholder={t("Search servers")} aria-label={t("Search servers")} value={search} onChange={(e) => setSearch(e.target.value)} />
          </InputGroup>
          <Button
            type="button"
            variant="outline"
            disabled={joinable.length === 0}
            onClick={() =>
              onChange(
                all
                  ? selected.filter((r) => !joinable.some((s) => key(refOf(s)) === key(r)))
                  : [...selected, ...joinable.map(refOf).filter((r) => !order.includes(key(r)))],
              )
            }
          >
            {all ? t("Clear") : t("Choose all ({{count}})", { count: joinable.length })}
          </Button>
        </div>
      )}
      <ul className="max-h-80 divide-y overflow-y-auto rounded-lg ring-1 ring-border">
        {sorted.length === 0 && <li className="p-4 text-center text-sm text-muted-foreground">{t("No server matches your search.")}</li>}
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
                  <span className="flex flex-wrap items-center gap-2 text-sm font-medium">
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
    </div>
  )
}
