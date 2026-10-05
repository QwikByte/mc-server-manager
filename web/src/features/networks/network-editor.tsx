import { useQuery } from "@tanstack/react-query"
import { useBlocker } from "@tanstack/react-router"
import { t } from "i18next"
import { useState } from "react"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Section } from "@/components/section"
import { Button } from "@/components/ui/button"
import { Field, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { useAccess } from "@/features/access/use-access"
import { nodeQuery } from "@/features/nodes/api"
import { useOperation } from "@/features/operations/use-operation"
import { allServersQuery } from "@/features/servers/api"
import { type Network, useUpdateNetwork } from "./api"
import { BackendList } from "./backend-list"
import { BedrockSection } from "./bedrock"
import { type Draft, draftOf, effects, settingsOf } from "./draft"
import { FirewallConfirmation, ForwardingChoice } from "./forwarding"
import { exposed, isValid } from "./problems"
import { Routing } from "./routing"
import { Topology } from "./topology"
import { findServer, hostOf, isBungee, key } from "./servers"
import { useNetworkUsage } from "./usage"

const same = (a: Draft, b: Draft) => JSON.stringify(a) === JSON.stringify(b)

/**
 * The network as a picture of where players go and which servers it has, edited as a whole:
 * saving configures the servers and makes the proxy reload its configuration.
 */
export function NetworkEditor({ network }: { network: Network }) {
  const editable = useAccess().can("networks.manage")
  const saved = draftOf(network)
  const [draft, setDraft] = useState(saved)
  const [seen, setSeen] = useState(saved)
  // Follows changes of the network made elsewhere, unless there are changes here.
  if (!same(saved, seen)) {
    setSeen(saved)
    if (same(draft, seen)) setDraft(saved)
  }
  const dirty = !same(draft, saved)
  const update = useUpdateNetwork(network.id)
  const operation = useOperation()
  const { data: servers } = useQuery(allServersQuery)
  const { data: proxyNode } = useQuery(nodeQuery(network.proxy.nodeId))
  const usage = useNetworkUsage([network])
  const bungee = isBungee(network.proxyType)
  const proxy = findServer(servers, network.proxy)
  const entries = new Map(draft.backends.map((b) => [key(b), { name: b.name, server: findServer(servers, b) }]))
  const blocker = useBlocker({ shouldBlockFn: () => dirty, enableBeforeUnload: () => dirty, withResolver: true })
  const change = (c: Partial<Draft>) => setDraft((d) => ({ ...d, ...c }))
  const proxyHost = hostOf(proxyNode?.address)
  const address = proxy && proxyHost ? `${proxyHost}:${proxy.port}` : undefined
  const bedrockAddress = proxyHost && draft.bedrockPort ? `${proxyHost}:${draft.bedrockPort}` : undefined

  function save() {
    operation.run((onStart) => update.mutateAsync({ settings: settingsOf(draft), onStart }), {
      title: t("Configuring {{name}}…", { name: draft.name }),
      notify: true,
      done: () => ({ message: t("Saved {{name}}", { name: draft.name }) }),
      then: (n) => setDraft(draftOf(n)),
    })
  }

  return (
    <div className="pb-28">
      <Topology network={network} draft={draft} servers={servers} usage={usage} address={address} bedrockAddress={bedrockAddress} />
      <Routing draft={draft} onChange={change} servers={entries} address={address} bungee={bungee} editable={editable} />
      <BackendList
        network={network}
        draft={draft}
        onChange={setDraft}
        servers={servers}
        usage={usage}
        proxyHost={proxyHost}
        editable={editable}
      />
      <Section title={t("Network")} description={t("The name of the network and how the proxy tells the servers who a player is.")}>
        <div className="surface grid gap-6 rounded-xl p-5 sm:p-6">
          <Field className="sm:max-w-sm">
            <FieldLabel htmlFor="network-name">{t("Name")}</FieldLabel>
            <Input id="network-name" value={draft.name} maxLength={64} disabled={!editable} onChange={(e) => change({ name: e.target.value })} />
          </Field>
          <ForwardingChoice value={draft.forwarding} bungee={bungee} disabled={!editable} onChange={(forwarding) => change({ forwarding })} />
          {exposed(network, draft) && (
            <FirewallConfirmation
              checked={draft.firewalled}
              proxyNode={proxyNode?.name}
              disabled={!editable}
              onChange={(firewalled) => change({ firewalled })}
            />
          )}
        </div>
      </Section>
      <BedrockSection
        network={network}
        draft={draft}
        onChange={change}
        servers={servers}
        proxyNode={proxyNode}
        proxyHost={proxyHost}
        editable={editable}
      />
      {dirty && <SaveBar network={network} draft={draft} saving={update.isPending} onDiscard={() => setDraft(saved)} onSave={save} />}
      <ConfirmDialog
        open={blocker.status === "blocked"}
        onOpenChange={(open) => !open && blocker.reset?.()}
        title={t("Discard your changes?")}
        description={t("Your changes to the network haven't been saved.")}
        action={t("Discard changes")}
        destructive
        onConfirm={() => blocker.proceed?.()}
      />
    </div>
  )
}

/** Tells what saving does to the servers, and saves or discards the changes. */
function SaveBar({
  network,
  draft,
  saving,
  onDiscard,
  onSave,
}: {
  network: Network
  draft: Draft
  saving: boolean
  onDiscard: () => void
  onSave: () => void
}) {
  const e = effects(network, draft, isBungee(network.proxyType))
  const names = (list: { name: string }[]) => list.map((b) => b.name).join(", ")
  const consequences = [
    e.restart.length > 0 && t("Restarts: {{names}}.", { names: names(e.restart) }),
    e.left.length > 0 && t("Accept players directly again, in online mode: {{names}}.", { names: names(e.left) }),
    e.bedrock === "on"
      ? t("The proxy restarts to load Geyser and Floodgate, which disconnects all players.")
      : e.bedrock === "off"
        ? t("The proxy restarts without Geyser and Floodgate, which disconnects all players.")
        : e.bedrock === "port"
          ? t("The proxy restarts to let Bedrock players in at the new port, which disconnects all players.")
          : e.proxyRestarts
            ? t("The proxy restarts, which disconnects all players, as BungeeCord can't reload without a server it had.")
            : t("The proxy reloads its configuration without disconnecting anyone."),
  ].filter(Boolean)

  return (
    <div className="sticky bottom-4 z-10 mt-10 flex flex-wrap items-center justify-between gap-3 rounded-2xl bg-popover/90 px-4 py-3 shadow-2xl ring-1 ring-foreground/10 backdrop-blur-xl">
      <div className="min-w-0 space-y-0.5">
        <p className="flex items-center gap-2.5 text-sm font-medium">
          <span aria-hidden className="size-2 rounded-full bg-warning" />
          {t("Unsaved changes")}
        </p>
        <p className="text-xs text-muted-foreground">{consequences.join(" ")}</p>
      </div>
      <div className="flex gap-2">
        <Button variant="ghost" onClick={onDiscard}>
          {t("Discard")}
        </Button>
        <Button disabled={saving || !isValid(network, draft)} onClick={onSave}>
          {t("Save and apply")}
        </Button>
      </div>
    </div>
  )
}
