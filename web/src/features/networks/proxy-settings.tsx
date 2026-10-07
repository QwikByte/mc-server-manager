import { FileTextIcon, LockSimpleIcon, MagnifyingGlassIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useBlocker } from "@tanstack/react-router"
import { t } from "i18next"
import { useState } from "react"
import { toast } from "sonner"
import { ErrorCallout } from "@/components/callout"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { EmptyState } from "@/components/empty-state"
import { Button } from "@/components/ui/button"
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { PropertyField } from "@/features/properties/property-field"
import { checkValue, iconMatches } from "@/features/properties/schema"
import { ServerIcon } from "@/features/properties/server-icon"
import { useServer } from "@/features/servers/api"
import { type ProxySettings, proxySettingsQuery, type ServerRef, type SettingValue, useUpdateProxySettings } from "./api"
import { definition, groups, order } from "./proxy-schema"
import { isBungee } from "./servers"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId/proxy")

/** The Configuration tab of a proxy server. */
export function ServerProxySettingsPage() {
  const { nodeId, serverId } = route.useParams()
  const { server } = useServer(nodeId, serverId)
  if (!server) return <Skeleton className="h-96 rounded-xl" />
  return <ProxySettingsEditor proxy={{ nodeId, serverId }} type={server.type} running={server.state !== "stopped"} />
}

/** The settings of a proxy in its own configuration file, velocity.toml or config.yml, as a form. */
export function ProxySettingsEditor({ proxy, type, running }: { proxy: ServerRef; type: string; running: boolean }) {
  const { data, isPending, error } = useQuery(proxySettingsQuery(proxy))
  if (isPending) return <Skeleton className="h-96 rounded-xl" />
  if (error) return <ErrorCallout error={error} />
  // The network may have written its part before the proxy ever started and wrote the rest.
  if (!data.exists || Object.keys(data.settings).length === 0)
    return (
      <EmptyState
        icon={FileTextIcon}
        tone="neutral"
        title={data.exists ? t("{{file}} has no settings yet", { file: data.file }) : t("No {{file}} yet", { file: data.file })}
        description={t("The proxy writes its settings when it starts for the first time. Start it once, then come back.")}
      />
    )
  return <ProxySettingsForm proxy={proxy} bungee={isBungee(type)} running={running} data={data} />
}

const toText = (value: SettingValue) => (Array.isArray(value) ? value.join("\n") : String(value))

/** Turns the text of a field back into a value of the kind the setting has. */
function fromText(current: SettingValue, text: string): SettingValue {
  if (typeof current === "boolean") return text === "true"
  if (typeof current === "number") return Number(text)
  if (Array.isArray(current)) return text.split("\n").map((s) => s.trim()).filter(Boolean)
  return text
}

function ProxySettingsForm({ proxy, bungee, running, data }: { proxy: ServerRef; bungee: boolean; running: boolean; data: ProxySettings }) {
  const [changes, setChanges] = useState<Record<string, string>>({})
  const [search, setSearch] = useState("")
  const { can } = useAccess()
  const update = useUpdateProxySettings(proxy)
  const count = Object.keys(changes).length
  const setting = (key: string) => definition(bungee, key, data.settings[key])
  const errors = Object.fromEntries(
    Object.entries(changes).flatMap(([key, value]) => {
      const error = checkValue(setting(key).kind, value)
      return error ? [[key, error]] : []
    }),
  )
  const blocker = useBlocker({ shouldBlockFn: () => count > 0, enableBeforeUnload: () => count > 0, withResolver: true })

  function set(key: string, value: string) {
    setChanges((current) => {
      const next = { ...current, [key]: value }
      if (value === toText(data.settings[key])) delete next[key] // back to the saved value
      return next
    })
  }

  function save() {
    const settings = Object.fromEntries(Object.entries(changes).map(([key, text]) => [key, fromText(data.settings[key], text)]))
    update.mutate(settings, {
      onSuccess: ({ reloaded }) => {
        setChanges({})
        toast.success(reloaded ? t("Saved. The proxy reloaded its configuration.") : t("Saved. The proxy reads it when it starts."))
      },
      onError: (e) => toast.error(e.message),
    })
  }

  const term = search.trim().toLowerCase()
  const keys = Object.keys(data.settings)
    .filter((key) => !term || [key, t(setting(key).label), t(setting(key).description)].some((s) => s.toLowerCase().includes(term)))
    .sort(order(bungee))
  const icon = can("files.write", proxy.nodeId, proxy.serverId) && iconMatches(term)

  return (
    <div className="pb-24">
      <InputGroup className="mb-8 w-full sm:max-w-xs">
        <InputGroupAddon>
          <MagnifyingGlassIcon />
        </InputGroupAddon>
        <InputGroupInput
          placeholder={t("Search settings")}
          aria-label={t("Search settings")}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </InputGroup>
      {keys.length === 0 && !icon && <p className="text-sm text-muted-foreground">{t("No setting matches your search.")}</p>}
      {groups.map((group) => {
        const inGroup = keys.filter((key) => setting(key).group === group)
        const withIcon = icon && group === "Server list"
        if (inGroup.length === 0 && !withIcon) return null
        return (
          <section key={group} className="surface mb-6 rounded-xl p-5 sm:p-6" aria-label={t(group)}>
            <h2 className="heading mb-5 text-base">{t(group)}</h2>
            <div className="grid gap-x-10 gap-y-6 md:grid-cols-2">
              {withIcon && <ServerIcon {...proxy} />}
              {inGroup.map((key) => (
                <PropertyField
                  key={key}
                  name={key}
                  setting={setting(key)}
                  value={changes[key] ?? toText(data.settings[key])}
                  error={errors[key]}
                  changed={key in changes}
                  note={key in changes && running && setting(key).restart ? t("Applies when the proxy starts again.") : undefined}
                  onChange={(value) => set(key, value)}
                />
              ))}
            </div>
          </section>
        )
      })}
      {data.locked.length > 0 && !term && (
        <section className="mb-6 rounded-xl border border-dashed p-5 sm:p-6" aria-label={t("Managed by the panel")}>
          <h2 className="heading mb-4 flex items-center gap-2 text-base">
            <LockSimpleIcon className="size-4 text-muted-foreground" />
            {t("Managed by the panel")}
          </h2>
          <dl className="grid gap-3 text-sm md:grid-cols-2">
            {data.locked.map(({ key, reason }) => (
              <div key={key}>
                <dt className="font-mono text-xs">{key}</dt>
                <dd className="text-muted-foreground">{reason}</dd>
              </div>
            ))}
          </dl>
        </section>
      )}
      {count > 0 && (
        <div className="sticky bottom-4 z-10 flex flex-wrap items-center justify-between gap-3 rounded-2xl bg-popover/90 px-4 py-3 shadow-2xl ring-1 ring-foreground/10 backdrop-blur-xl">
          <p className="flex items-center gap-2.5 text-sm font-medium">
            <span aria-hidden className="size-2 rounded-full bg-warning" />
            {t("{{count}} unsaved changes", { count, defaultValue_one: "{{count}} unsaved change" })}
            {running && <span className="font-normal text-muted-foreground">{t("The proxy reloads them without disconnecting anyone.")}</span>}
          </p>
          <div className="flex gap-2">
            <Button variant="ghost" onClick={() => setChanges({})}>
              {t("Discard")}
            </Button>
            <Button disabled={update.isPending || Object.keys(errors).length > 0} onClick={save}>
              {running ? t("Save and reload") : t("Save")}
            </Button>
          </div>
        </div>
      )}
      <ConfirmDialog
        open={blocker.status === "blocked"}
        onOpenChange={(open) => !open && blocker.reset?.()}
        title={t("Discard your changes?")}
        description={t("Your changes to the settings haven't been saved.")}
        action={t("Discard changes")}
        destructive
        onConfirm={() => blocker.proceed?.()}
      />
    </div>
  )
}
