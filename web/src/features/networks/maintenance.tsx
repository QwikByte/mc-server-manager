import { FileTextIcon, PlusIcon, WrenchIcon, XIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { ErrorCallout } from "@/components/callout"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Section } from "@/components/section"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { FieldError } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { useOperation } from "@/features/operations/use-operation"
import { allServersQuery } from "@/features/servers/api"
import { maintenanceQuery, type Network, useMaintenancePlayer, useSetMaintenance } from "./api"
import { findServer, isBungee } from "./servers"

/** Maintenance of a network with the Maintenance plugin on its proxy: on or off, and who may join meanwhile. */
export function MaintenanceSection({ network }: { network: Network }) {
  const { can } = useAccess()
  const manage = can("networks.manage")
  const { data: m, isPending, error } = useQuery(maintenanceQuery(network.id))
  // The list of servers is read more often, so it tells first when the proxy starts or stops.
  const proxy = findServer(useQuery(allServersQuery).data, network.proxy)
  const proxyRunning = proxy ? proxy.state === "running" : !!m?.proxyRunning
  const set = useSetMaintenance(network.id)
  const operation = useOperation()
  const [confirm, setConfirm] = useState(false)
  const folder = isBungee(network.proxyType) ? "plugins/Maintenance" : "plugins/maintenance"
  const { nodeId, serverId } = network.proxy

  function toggle(enabled: boolean) {
    const name = network.name
    operation.run((onStart) => set.mutateAsync({ enabled, onStart }), {
      title: enabled ? t("Turn on maintenance of {{name}}", { name }) : t("Turn off maintenance of {{name}}", { name }),
      notify: true,
      done: () => ({ message: enabled ? t("Maintenance of {{name}} is on", { name }) : t("Maintenance of {{name}} is off", { name }) }),
    })
  }

  return (
    <Section
      title={t("Maintenance")}
      description={t("Only the team may join, and the server list shows that the network is in maintenance.")}
      className="mt-0 mb-10"
      actions={
        manage &&
        m && (
          <Button
            variant={m.enabled ? "default" : "outline"}
            disabled={set.isPending || !proxyRunning}
            title={proxyRunning ? undefined : t("Start the proxy first.")}
            onClick={() => (m.enabled ? toggle(false) : setConfirm(true))}
          >
            <WrenchIcon />
            {m.enabled ? t("End maintenance") : t("Start maintenance…")}
          </Button>
        )
      }
    >
      {isPending ? (
        <Skeleton className="h-24 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : (
        <div className="surface grid gap-4 rounded-xl p-5">
          <div className="flex flex-wrap items-center gap-3 text-sm">
            <Pill tone={m.enabled ? "warning" : "neutral"}>{m.enabled ? t("In maintenance") : t("Open to all players")}</Pill>
            {!proxyRunning && <span className="text-muted-foreground">{t("The proxy isn't running.")}</span>}
            {!m.installed && proxyRunning && (
              <span className="text-muted-foreground">{t("The first time, the panel installs the Maintenance plugin on the proxy.")}</span>
            )}
          </div>
          {m.installed && (
            <>
              <Team network={network} players={m.players} editable={manage && proxyRunning} />
              {can("files.read", nodeId, serverId) && (
                <p className="text-sm text-muted-foreground">
                  <Link
                    to="/nodes/$nodeId/servers/$serverId/files"
                    params={{ nodeId, serverId }}
                    search={{ path: folder, edit: `${folder}/config.yml` }}
                    className="inline-flex items-center gap-1.5 font-medium text-foreground hover:underline"
                  >
                    <FileTextIcon />
                    {t("Change the texts")}
                  </Link>{" "}
                  {t("The message in the server list and the one for players who may not join are in the plugin's config.yml.")}
                </p>
              )}
            </>
          )}
        </div>
      )}
      {confirm && m && (
        <ConfirmDialog
          open
          onOpenChange={setConfirm}
          title={t("Start maintenance of {{name}}?", { name: network.name })}
          description={
            m.installed
              ? t("Players who aren't in the team leave the network and can't join until maintenance ends.")
              : t(
                  "The panel installs the Maintenance plugin from Modrinth on the proxy and restarts the proxy to load it, which disconnects all players once. Then only the team may join.",
                )
          }
          action={t("Start maintenance")}
          onConfirm={() => toggle(true)}
        />
      )}
    </Section>
  )
}

/** The players who may join during maintenance. */
function Team({ network, players, editable }: { network: Network; players: { name: string }[]; editable: boolean }) {
  const change = useMaintenancePlayer(network.id)
  const [name, setName] = useState("")

  function add(event: FormEvent) {
    event.preventDefault()
    change.mutate({ name: name.trim(), add: true }, { onSuccess: () => setName("") })
  }

  return (
    <div className="grid gap-2">
      <p className="text-sm font-medium">{t("Team")}</p>
      <div className="flex flex-wrap gap-2">
        {players.length === 0 && (
          <span className="text-sm text-muted-foreground">
            {t("Nobody yet: only players with the permission maintenance.bypass may join.")}
          </span>
        )}
        {players.map((p) => (
          <span key={p.name} className="inline-flex items-center gap-1 rounded-md bg-muted py-1 pr-1 pl-2 font-mono text-xs font-medium">
            {p.name}
            {editable && (
              <Button
                variant="ghost"
                size="icon-xs"
                aria-label={t("Remove {{name}}", { name: p.name })}
                disabled={change.isPending}
                onClick={() => change.mutate({ name: p.name, add: false })}
              >
                <XIcon />
              </Button>
            )}
          </span>
        ))}
      </div>
      {editable && (
        <form onSubmit={add} className="flex max-w-sm gap-2">
          <Input
            aria-label={t("Player")}
            placeholder={t("Name of a player")}
            maxLength={17}
            pattern="\.?[A-Za-z0-9_]{1,16}"
            className="font-mono"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <Button type="submit" variant="outline" disabled={!name.trim() || change.isPending}>
            <PlusIcon />
            {t("Add")}
          </Button>
        </form>
      )}
      {change.error && <FieldError>{change.error.message}</FieldError>}
    </div>
  )
}
