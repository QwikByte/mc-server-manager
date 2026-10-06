import { ArrowRightIcon, CaretLeftIcon, GlobeIcon, PlusIcon, TrashIcon, UsersThreeIcon, XIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { type FormEvent, type ReactNode, useState } from "react"
import { IconTile } from "@/components/icon-tile"
import { Section } from "@/components/section"
import { StatusDot } from "@/components/status"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { FieldError } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import type { NodeServer } from "@/features/servers/api"
import { statusOf } from "@/features/servers/server-types"
import { cn } from "@/lib/utils"
import type { Draft } from "./draft"
import { hostError } from "./problems"

/** A server of the network as routes show it: the name players use, and the server if it is reachable. */
interface Entry {
  name: string
  server: NodeServer | null | undefined
}

/**
 * Where players go: everyone to the servers they join and fall back to, and those who
 * connect through a host name to its servers. This is the list Velocity calls try and
 * BungeeCord priorities, and their forced hosts.
 */
export function Routing({
  draft,
  onChange,
  servers,
  address,
  bungee,
  editable,
}: {
  draft: Draft
  onChange: (change: Partial<Draft>) => void
  /** The servers of the network by key. */
  servers: Map<string, Entry>
  /** Where players connect to the proxy, e.g. 203.0.113.10:25577. */
  address?: string
  bungee: boolean
  editable: boolean
}) {
  const hosts = draft.forcedHosts
  const setHost = (i: number, host: Partial<(typeof hosts)[number]>) =>
    onChange({ forcedHosts: hosts.map((h, j) => (i === j ? { ...h, ...host } : h)) })

  return (
    <Section title={t("Routing")} description={t("Where players go when they join, and where they fall back to when a server goes down.")}>
      <ol className="surface divide-y overflow-hidden rounded-xl">
        <Route
          icon={<IconTile icon={UsersThreeIcon} tone="info" size="sm" />}
          title={t("All players")}
          detail={address ? t("Connect to {{address}}", { address }) : undefined}
        >
          <Chain label={t("Servers all players try")} names={draft.try} servers={servers} editable={editable} onChange={(names) => onChange({ try: names })} />
          {draft.try.length === 0 && <FieldError>{t("Choose at least one server that players join.")}</FieldError>}
        </Route>
        {hosts.map((h, i) => (
          <Route
            key={i}
            icon={<IconTile icon={GlobeIcon} tone="violet" size="sm" />}
            title={
              editable ? (
                <Input
                  value={h.host}
                  aria-label={t("Host name")}
                  aria-invalid={!!hostError(hosts, i)}
                  className="h-8 font-mono text-sm"
                  onChange={(e) => setHost(i, { host: e.target.value.trim().toLowerCase() })}
                />
              ) : (
                <span className="font-mono">{h.host}</span>
              )
            }
            action={
              editable && (
                <Button
                  size="icon-sm"
                  variant="ghost"
                  className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                  aria-label={t("Remove {{host}}", { host: h.host })}
                  onClick={() => onChange({ forcedHosts: hosts.filter((_, j) => j !== i) })}
                >
                  <TrashIcon />
                </Button>
              )
            }
          >
            <Chain
              label={t("Servers of {{host}}", { host: h.host })}
              names={h.servers}
              servers={servers}
              max={bungee ? 1 : undefined}
              editable={editable}
              onChange={(names) => setHost(i, { servers: names })}
            />
            {hostError(hosts, i) && <FieldError>{hostError(hosts, i)}</FieldError>}
          </Route>
        ))}
        {editable && (
          <li className="p-4">
            <AddHost taken={hosts.map((h) => h.host)} onAdd={(host) => onChange({ forcedHosts: [...hosts, { host, servers: draft.try.slice(0, 1) }] })} />
          </li>
        )}
      </ol>
      <p className="mt-3 text-xs text-muted-foreground">
        {bungee
          ? t("BungeeCord sends players who connect through a host name to its server, and otherwise to the first server they can join.")
          : t("Players try the servers from left to right. If one is offline or full, or kicks them, they continue with the next.")}
      </p>
    </Section>
  )
}

function Route({ icon, title, detail, action, children }: { icon: ReactNode; title: ReactNode; detail?: string; action?: ReactNode; children: ReactNode }) {
  return (
    <li className="grid gap-3 p-4 md:grid-cols-[16rem_auto_1fr] md:items-center">
      <div className="flex min-w-0 items-center gap-3">
        {icon}
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-semibold">{title}</div>
          {detail && <p className="truncate font-mono text-xs text-muted-foreground">{detail}</p>}
        </div>
        <span className="md:hidden">{action}</span>
      </div>
      <ArrowRightIcon aria-hidden className="hidden size-4 text-muted-foreground/60 md:block" weight="bold" />
      <div className="flex min-w-0 items-start justify-between gap-3">
        <div className="min-w-0 space-y-2">{children}</div>
        <span className="max-md:hidden">{action}</span>
      </div>
    </li>
  )
}

/**
 * Servers tried in this order. They are dragged into another order, or moved forward with
 * their button, which the keyboard reaches too.
 */
function Chain({
  label,
  names,
  servers,
  max,
  editable,
  onChange,
}: {
  label: string
  names: string[]
  servers: Map<string, Entry>
  max?: number
  editable: boolean
  onChange: (names: string[]) => void
}) {
  const [dragged, setDragged] = useState<number>()
  const rest = [...servers.keys()].filter((name) => !names.includes(name))
  const move = (from: number, to: number) => {
    const next = [...names]
    next.splice(to, 0, ...next.splice(from, 1))
    onChange(next)
  }

  return (
    <ol aria-label={label} className="flex flex-wrap items-center gap-1.5">
      {names.map((id, i) => {
        const { name, server } = servers.get(id) ?? { name: id, server: undefined }
        return (
          <li key={id} className="flex items-center gap-1.5">
            {i > 0 && <ArrowRightIcon aria-label={t("then")} className="size-3.5 text-muted-foreground/60" />}
            <span
              draggable={editable}
              onDragStart={() => setDragged(i)}
              onDragEnd={() => setDragged(undefined)}
              onDragOver={(e) => dragged !== undefined && e.preventDefault()}
              onDrop={() => dragged !== undefined && dragged !== i && move(dragged, i)}
              className={cn(
                "inline-flex h-8 items-center gap-2 rounded-lg bg-muted pr-1 pl-2 text-sm font-medium ring-1 ring-foreground/5 ring-inset",
                editable && "cursor-grab active:cursor-grabbing",
                dragged === i && "opacity-50",
              )}
            >
              <span className="grid size-5 place-items-center rounded-md bg-card text-[0.6875rem] font-bold tabular-nums">{i + 1}</span>
              {server ? <StatusDot status={statusOf(server)} label={t(statusOf(server).label)} /> : null}
              <span className="font-mono text-xs">{name}</span>
              {editable && i > 0 && (
                <button
                  type="button"
                  aria-label={t("Try {{name}} earlier", { name })}
                  title={t("Try earlier")}
                  onClick={() => move(i, i - 1)}
                  className="grid size-6 place-items-center rounded-md text-muted-foreground hover:bg-card hover:text-foreground"
                >
                  <CaretLeftIcon className="size-3.5" />
                </button>
              )}
              {editable && (
                <button
                  type="button"
                  aria-label={t("Remove {{name}} from here", { name })}
                  title={t("Remove")}
                  onClick={() => onChange(names.filter((n) => n !== id))}
                  className="grid size-6 place-items-center rounded-md text-muted-foreground hover:bg-card hover:text-foreground"
                >
                  <XIcon className="size-3.5" />
                </button>
              )}
            </span>
          </li>
        )
      })}
      {editable && rest.length > 0 && (max === undefined || names.length < max) && (
        <li className="flex items-center gap-1.5">
          {names.length > 0 && <ArrowRightIcon aria-hidden className="size-3.5 text-muted-foreground/40" />}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button size="sm" variant="outline" className="border-dashed">
                <PlusIcon />
                {names.length === 0 ? t("Choose server") : t("Fallback")}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              {rest.map((id) => (
                <DropdownMenuItem key={id} onSelect={() => onChange([...names, id])}>
                  <span className="font-mono text-xs">{servers.get(id)?.name}</span>
                  <span className="text-muted-foreground">{servers.get(id)?.server?.name}</span>
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </li>
      )}
    </ol>
  )
}

/** Adds a host name, e.g. survival.example.com, whose players join their own servers. */
function AddHost({ taken, onAdd }: { taken: string[]; onAdd: (host: string) => void }) {
  const [host, setHost] = useState("")
  const error = host ? hostError([...taken.map((h) => ({ host: h, servers: ["-"] })), { host, servers: ["-"] }], taken.length) : undefined

  function submit(event: FormEvent) {
    event.preventDefault()
    if (!host || error) return
    onAdd(host)
    setHost("")
  }

  return (
    <form onSubmit={submit} className="flex flex-wrap items-start gap-2">
      <div className="min-w-0 flex-1 sm:max-w-sm">
        <Input
          value={host}
          placeholder={t("e.g. survival.example.com")}
          aria-label={t("New host name")}
          aria-invalid={!!error}
          className="font-mono text-sm"
          onChange={(e) => setHost(e.target.value.trim().toLowerCase())}
        />
        {error && <FieldError className="mt-1.5">{error}</FieldError>}
      </div>
      <Button type="submit" variant="outline" disabled={!host || !!error}>
        <GlobeIcon />
        {t("Add host name")}
      </Button>
    </form>
  )
}
