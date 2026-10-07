import { CaretDownIcon, CheckCircleIcon, DownloadSimpleIcon, WarningCircleIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { type ReactNode, useId, useState } from "react"
import { Callout } from "@/components/callout"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { FieldError } from "@/components/ui/field"
import { useAccess } from "@/features/access/use-access"
import { networksQuery, type ServerRef } from "@/features/networks/api"
import { key, refOf } from "@/features/networks/servers"
import { allServersQuery, type NodeServer } from "@/features/servers/api"
import { displayVersion, serverType } from "@/features/servers/server-types"
import { OperationStatus } from "@/features/operations/operation-status"
import { mergeResults } from "@/features/operations/retry"
import { RetryButton } from "@/features/operations/retry-button"
import { guard, useOperation } from "@/features/operations/use-operation"
import { fallback, type InstallResult, maxServers, type ProjectVersion, type SearchHit, supports, useInstallPlugins } from "./api"
import { ChannelPill } from "./channel-pill"
import { PluginIcon } from "./plugin-icon"
import { VersionMenu } from "./version-menu"

/**
 * Installs a plugin or mod on any number of servers it runs on, also on the game servers or the proxy of a network at
 * once.
 */
export function InstallDialog({ hit }: { hit: SearchHit }) {
  const [open, setOpen] = useState(false)
  const [selected, setSelected] = useState<string[]>([])
  const [pinned, setPinned] = useState<ProjectVersion>()
  // The batch that runs, of how many, e.g. 1 of 2 for 150 servers.
  const [[part, parts], setPart] = useState([0, 0])
  const { can } = useAccess()
  const { data: servers = [] } = useQuery({ ...allServersQuery, enabled: open })
  const { data: networks = [] } = useQuery({ ...networksQuery, enabled: open && can("networks.view") })
  const install = useInstallPlugins()
  const operation = useOperation()
  const suitable = servers.filter((s) => supports(hit.loaders, s.type) && can("plugins.manage", s.nodeId, s.id))
  const picked = new Set(selected)
  const chosen = suitable.filter((s) => picked.has(key(refOf(s))))
  // A version can be chosen for servers that run the same software and Minecraft version.
  const same = chosen.length > 0 && chosen.every((s) => s.type === chosen[0].type && s.version === chosen[0].version)
  const choose = (keys: string[], on: boolean) => {
    setSelected((list) => (on ? [...new Set([...list, ...keys])] : list.filter((k) => !keys.includes(k))))
    setPinned(undefined)
  }
  // The servers of networks that can run the project, to choose them at once.
  const groups = networks.flatMap((n) => {
    const of = (refs: ServerRef[]) => {
      const keys = new Set(refs.map(key))
      return suitable.filter((s) => keys.has(key(refOf(s))))
    }
    return [
      { id: `${n.id}/servers`, label: t("Game servers of {{network}}", { network: n.name }), servers: of(n.backends) },
      { id: `${n.id}/proxy`, label: t("Proxy of {{network}}", { network: n.name }), servers: of([n.proxy]) },
    ].filter((g) => g.servers.length > 0)
  })

  const title = t("Install {{name}}", { name: hit.title })

  const versions = pinned && { [hit.id]: pinned.id }
  // The results of a retry take the place of those of the servers it tried again.
  const [earlier, setEarlier] = useState<InstallResult[]>()
  const results = install.data && mergeResults(earlier, install.data)

  function start(servers = chosen.map(refOf)) {
    operation.run(
      (onStart) =>
        install.mutateAsync({
          projects: [hit.id],
          servers,
          versions,
          onStart: (op, batch) => {
            setPart([batch + 1, Math.ceil(servers.length / maxServers)])
            onStart(op)
          },
        }),
      {
        title,
        done: (results) => {
          const ok = results.filter((r) => !r.error).length
          const preReleases = results.some((r) => r.installed.some((f) => fallback(f, versions)))
          return {
            message:
              ok === results.length
                ? t("Installed {{name}} on {{count}} servers", {
                    name: hit.title,
                    count: ok,
                    defaultValue_one: "Installed {{name}} on {{count}} server",
                  })
                : t("Installed {{name}} on {{ok}} of {{count}} servers", { name: hit.title, ok, count: results.length }),
            description: preReleases ? t("Where no release suits a server, a pre-release was installed.") : undefined,
            warning: ok < results.length || preReleases,
          }
        },
      },
    )
  }

  function onOpenChange(next: boolean) {
    setOpen(next)
    if (!next) {
      install.reset()
      operation.reset()
      setSelected([])
      setPinned(undefined)
      setEarlier(undefined)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline">
          <DownloadSimpleIcon />
          {t("Install")}
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg" {...guard(install.isPending)}>
        {operation.live && !install.data ? (
          <OperationStatus
            op={operation.live}
            title={parts > 1 ? t("{{title}}, part {{part}} of {{parts}}", { title, part, parts }) : title}
            onBackground={() => {
              operation.background(title)
              onOpenChange(false)
            }}
            onBack={() => {
              operation.reset()
              install.reset()
              setEarlier(undefined)
            }}
          />
        ) : (
          <>
            <DialogHeader className="flex-row items-center gap-3">
              <PluginIcon src={hit.icon} />
              <div className="min-w-0 space-y-1 text-left">
                <DialogTitle>{t("Install {{name}}", { name: hit.title })}</DialogTitle>
                <DialogDescription>{t("The newest release that suits each server is installed, with what it requires.")}</DialogDescription>
              </div>
            </DialogHeader>
            {results ? (
              <Results results={results} servers={servers} versions={versions} />
            ) : suitable.length === 0 ? (
              <p className="py-4 text-sm text-muted-foreground">{t("None of your servers can run {{name}}.", { name: hit.title })}</p>
            ) : (
              <div className="-mx-1 grid max-h-96 gap-4 overflow-y-auto px-1">
                {groups.length > 0 && (
                  <Group title={t("Networks")}>
                    {groups.map((g) => {
                      const keys = g.servers.map((s) => key(refOf(s)))
                      const count = keys.filter((k) => picked.has(k)).length
                      return (
                        <Row
                          key={g.id}
                          checked={count === 0 ? false : count === keys.length || "indeterminate"}
                          onChange={(on) => choose(keys, on)}
                          title={g.label}
                          detail={
                            g.servers.length === 1
                              ? g.servers[0].name
                              : t("{{count}} servers", { count: g.servers.length, defaultValue_one: "{{count}} server" })
                          }
                        />
                      )
                    })}
                  </Group>
                )}
                <Group title={groups.length > 0 ? t("Servers") : undefined}>
                  {suitable.map((s) => (
                    <Row
                      key={key(refOf(s))}
                      checked={picked.has(key(refOf(s)))}
                      onChange={(on) => choose([key(refOf(s))], on)}
                      title={s.name}
                      detail={`${serverType(s.type).label} ${displayVersion(s.version)} · ${s.nodeName}`}
                    />
                  ))}
                </Group>
              </div>
            )}
            {!install.data && chosen.length > 0 && (
              <div className="flex items-center justify-between gap-3 rounded-lg bg-muted/50 px-3 py-2 text-sm">
                <span className="font-medium">{t("Version")}</span>
                {same ? (
                  <VersionMenu
                    project={hit.id}
                    title={hit.title}
                    type={chosen[0].type}
                    version={chosen[0].version}
                    current={pinned?.id}
                    onNewest={() => setPinned(undefined)}
                    onPick={setPinned}
                  >
                    <Button size="sm" variant="outline">
                      {pinned ? <span className="font-mono">{pinned.number}</span> : t("Newest suitable release")}
                      <CaretDownIcon />
                    </Button>
                  </VersionMenu>
                ) : (
                  <span className="text-right text-xs text-muted-foreground">
                    {t("The newest suitable release, as the servers run different software or Minecraft versions.")}
                  </span>
                )}
              </div>
            )}
            {install.error && <FieldError>{install.error.message}</FieldError>}
            <DialogFooter>
              {results && (
                <RetryButton
                  results={results}
                  onRetry={(failed) => {
                    setEarlier(results)
                    start(failed)
                  }}
                />
              )}
              <DialogClose asChild>
                <Button variant="outline" disabled={install.isPending}>
                  {install.data ? t("Done") : t("Cancel")}
                </Button>
              </DialogClose>
              {!install.data && (
                <Button disabled={chosen.length === 0 || install.isPending} onClick={() => start()}>
                  {install.isPending
                    ? t("Installing…")
                    : t("Install on {{count}} servers", { count: chosen.length, defaultValue_one: "Install on {{count}} server" })}
                </Button>
              )}
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}

/** Servers to tick, under a title. */
function Group({ title, children }: { title?: string; children: ReactNode }) {
  const id = useId()
  return (
    <div className="grid gap-1.5">
      {title && (
        <p id={id} className="text-xs font-medium text-muted-foreground">
          {title}
        </p>
      )}
      <ul aria-labelledby={title && id} className="grid gap-1">
        {children}
      </ul>
    </div>
  )
}

/** A server, or the servers of a network, to tick; a network of which only some are ticked shows a mixed state. */
function Row({
  checked,
  onChange,
  title,
  detail,
}: {
  checked: boolean | "indeterminate"
  onChange: (on: boolean) => void
  title: string
  detail: string
}) {
  return (
    <li>
      <label className="flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2 ring-1 ring-foreground/8 hover:bg-muted/50">
        <Checkbox checked={checked} onCheckedChange={(on) => onChange(on === true)} />
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium">{title}</span>
          <span className="block truncate text-xs text-muted-foreground">{detail}</span>
        </span>
      </label>
    </li>
  )
}

/** What was installed on each server, with the pre-releases installed where no release suits it. Servers load new plugins when they restart. */
function Results({ results, servers, versions }: { results: InstallResult[]; servers: NodeServer[]; versions?: Record<string, string> }) {
  const preReleases = results.some((r) => r.installed.some((f) => fallback(f, versions)))
  return (
    <div className="grid gap-3">
      <ul className="grid gap-2">
        {results.map((r) => (
          <li key={key(r)} className="flex items-start gap-3 rounded-lg px-3 py-2 text-sm ring-1 ring-foreground/8">
            {r.error ? (
              <WarningCircleIcon className="mt-0.5 size-4 shrink-0 text-destructive" weight="fill" />
            ) : (
              <CheckCircleIcon className="mt-0.5 size-4 shrink-0 text-success" weight="fill" />
            )}
            <span className="min-w-0">
              <span className="block font-medium">{servers.find((s) => key(refOf(s)) === key(r))?.name ?? r.serverId}</span>
              {r.error ? (
                <span className="block text-xs text-muted-foreground">{r.error}</span>
              ) : (
                <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
                  {r.installed.map((f) => (
                    <span key={f.fileName} className="inline-flex items-center gap-1.5">
                      {f.fileName}
                      <ChannelPill channel={f.channel} />
                    </span>
                  ))}
                </span>
              )}
            </span>
          </li>
        ))}
      </ul>
      {preReleases && (
        <Callout tone="warning">{t("No release suits some of the servers, so a pre-release was installed there. Test it before you use it in production.")}</Callout>
      )}
      <p className="text-xs text-muted-foreground">{t("Restart the servers to load what was installed.")}</p>
    </div>
  )
}
