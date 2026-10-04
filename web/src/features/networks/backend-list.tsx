import { CubeIcon, HardDrivesIcon, HashIcon, PuzzlePieceIcon, ShieldWarningIcon, TrashIcon, UsersIcon } from "@phosphor-icons/react"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { CopyField } from "@/components/copy-field"
import { IconTile } from "@/components/icon-tile"
import { Section } from "@/components/section"
import { Chip } from "@/components/chip"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import type { NodeServer } from "@/features/servers/api"
import { displayVersion, serverLook, serverType } from "@/features/servers/server-types"
import type { ServerUsage } from "@/features/usage/api"
import { AddBackendDialog } from "./add-backend-dialog"
import type { Backend, Network } from "./api"
import { type Draft, withoutBackend } from "./draft"
import { nameError } from "./problems"
import { ServerLabel } from "./server-label"
import { findServer, firewallCommand, forwardingMod, key } from "./servers"

/** The game servers behind the proxy, with the name players use and their settings. */
export function BackendList({
  network,
  draft,
  onChange,
  servers,
  usage,
  proxyHost,
  editable,
}: {
  network: Network
  draft: Draft
  onChange: (draft: Draft) => void
  servers?: NodeServer[]
  usage: (b: Backend) => ServerUsage | undefined
  /** The address of the proxy's node, which servers on other nodes must let in. */
  proxyHost?: string
  editable: boolean
}) {
  const bungee = network.proxyType === "bungeecord" || network.proxyType === "waterfall"
  const set = (i: number, change: Partial<Backend>) =>
    onChange({ ...draft, backends: draft.backends.map((b, j) => (i === j ? { ...b, ...change } : b)) })

  return (
    <Section
      title={t("Servers")}
      description={t("Players switch between them with /server and their name.")}
      actions={editable && <AddBackendDialog network={network} draft={draft} onAdd={onChange} />}
    >
      <ul className="grid gap-4 lg:grid-cols-2 2xl:grid-cols-3">
        {draft.backends.map((backend, i) => {
          const server = findServer(servers, backend)
          const k = key(backend)
          const position = draft.try.indexOf(k)
          const hosts = draft.forcedHosts.filter((h) => h.servers.includes(k)).map((h) => h.host)
          const remote = backend.nodeId !== network.proxy.nodeId
          const players = usage(backend)?.players
          const mod = server && forwardingMod(server.type)
          const error = nameError(draft.backends, i)
          return (
            <li key={k} className="surface flex flex-col gap-4 rounded-xl p-5">
              <div className="flex items-start gap-3">
                <IconTile {...(server ? serverLook(server.type) : { icon: CubeIcon })} />
                <div className="min-w-0 flex-1">
                  <Link
                    to="/nodes/$nodeId/servers/$serverId"
                    params={{ nodeId: backend.nodeId, serverId: backend.serverId }}
                    className="hover:underline"
                  >
                    <ServerLabel server={server} />
                  </Link>
                  <div className="mt-1.5 flex flex-wrap gap-1.5">
                    {position === 0 && <Pill tone="info">{t("Players join here")}</Pill>}
                    {position > 0 && <Pill tone="neutral">{t("Fallback {{position}}", { position })}</Pill>}
                    {hosts.map((host) => (
                      <Pill key={host} tone="violet">
                        <span className="font-mono">{host}</span>
                      </Pill>
                    ))}
                  </div>
                </div>
                {editable && (
                  <Button
                    size="icon-sm"
                    variant="ghost"
                    className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                    aria-label={t("Remove {{name}} from the network", { name: server?.name ?? backend.name })}
                    title={draft.backends.length === 1 ? t("A network needs at least one server") : t("Remove from the network")}
                    disabled={draft.backends.length === 1}
                    onClick={() => onChange(withoutBackend(draft, k))}
                  >
                    <TrashIcon />
                  </Button>
                )}
              </div>
              <div className="flex flex-wrap gap-1.5">
                {server && (
                  <Chip>
                    {serverType(server.type).label} {displayVersion(server.version)}
                  </Chip>
                )}
                {server && <Chip icon={HardDrivesIcon}>{server.nodeName}</Chip>}
                {server && (
                  <Chip icon={HashIcon}>
                    <span className="font-mono">{remote ? server.port : t("internal")}</span>
                  </Chip>
                )}
                {players && <Chip icon={UsersIcon}>{t("{{count}} players", { count: players.online, defaultValue_one: "{{count}} player" })}</Chip>}
                {mod && <Chip icon={PuzzlePieceIcon}>{mod}</Chip>}
              </div>
              <Field data-invalid={!!error}>
                <FieldLabel htmlFor={`backend-${k}`}>{t("Name in the network")}</FieldLabel>
                <div className="flex items-center rounded-md ring-1 ring-input focus-within:ring-2 focus-within:ring-ring">
                  <span className="pl-3 font-mono text-sm text-muted-foreground">/server</span>
                  <Input
                    id={`backend-${k}`}
                    value={backend.name}
                    disabled={!editable}
                    aria-invalid={!!error}
                    className="border-0 font-mono shadow-none ring-0 focus-visible:ring-0"
                    onChange={(e) => set(i, { name: e.target.value.toLowerCase() })}
                  />
                </div>
                {error && <FieldError>{error}</FieldError>}
              </Field>
              {bungee && (
                <>
                  <Field>
                    <FieldLabel htmlFor={`motd-${k}`}>{t("MOTD for its host names")}</FieldLabel>
                    <Input
                      id={`motd-${k}`}
                      value={backend.motd}
                      maxLength={256}
                      placeholder={t("The proxy's MOTD")}
                      disabled={!editable}
                      className="font-mono"
                      onChange={(e) => set(i, { motd: e.target.value })}
                    />
                  </Field>
                  <Field orientation="horizontal">
                    <Switch id={`restricted-${k}`} checked={backend.restricted} disabled={!editable} onCheckedChange={(restricted) => set(i, { restricted })} />
                    <div>
                      <FieldLabel htmlFor={`restricted-${k}`}>{t("Restricted")}</FieldLabel>
                      <FieldDescription>
                        {t("Only players with the permission bungeecord.server.{{name}} may join.", { name: backend.name })}
                      </FieldDescription>
                    </div>
                  </Field>
                </>
              )}
              {remote && draft.forwarding === "legacy" && server && proxyHost && (
                <div className="space-y-2 rounded-lg bg-warning/10 p-3 text-xs ring-1 ring-warning/20 ring-inset">
                  <p className="flex items-center gap-2 font-medium text-warning">
                    <ShieldWarningIcon className="size-4" weight="duotone" />
                    {t("Let only the proxy reach port {{port}} on {{node}}:", { port: server.port, node: server.nodeName })}
                  </p>
                  <CopyField value={firewallCommand(server.port, proxyHost)} label={t("firewall command")} prefix="#" />
                </div>
              )}
            </li>
          )
        })}
      </ul>
    </Section>
  )
}
