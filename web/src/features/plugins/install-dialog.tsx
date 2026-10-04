import { CaretDownIcon, CheckCircleIcon, DownloadSimpleIcon, WarningCircleIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { useState } from "react"
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
import { key, refOf } from "@/features/networks/servers"
import { allServersQuery, type NodeServer } from "@/features/servers/api"
import { displayVersion, serverType } from "@/features/servers/server-types"
import { OperationStatus } from "@/features/operations/operation-status"
import { guard, useOperation } from "@/features/operations/use-operation"
import { type InstallResult, type ProjectVersion, type SearchHit, supports, useInstallPlugins } from "./api"
import { PluginIcon } from "./plugin-icon"
import { VersionMenu } from "./version-menu"

/** Installs a plugin or mod on any number of servers it runs on. */
export function InstallDialog({ hit }: { hit: SearchHit }) {
  const [open, setOpen] = useState(false)
  const [selected, setSelected] = useState<string[]>([])
  const [pinned, setPinned] = useState<ProjectVersion>()
  const { data: servers = [] } = useQuery({ ...allServersQuery, enabled: open })
  const install = useInstallPlugins()
  const operation = useOperation()
  const { can } = useAccess()
  const suitable = servers.filter((s) => supports(hit.loaders, s.type) && can("plugins.manage", s.nodeId, s.id))
  const chosen = suitable.filter((s) => selected.includes(key(refOf(s))))
  // A version can be chosen for servers that run the same software and Minecraft version.
  const same = chosen.length > 0 && chosen.every((s) => s.type === chosen[0].type && s.version === chosen[0].version)
  const toggle = (k: string, on: boolean) => {
    setSelected((list) => (on ? [...list, k] : list.filter((s) => s !== k)))
    setPinned(undefined)
  }

  const title = t("Install {{name}}", { name: hit.title })

  function start() {
    operation.run(
      (onStart) =>
        install.mutateAsync({ projects: [hit.id], servers: chosen.map(refOf), versions: pinned && { [hit.id]: pinned.id }, onStart }),
      {
        title,
        done: (results) => {
          const ok = results.filter((r) => !r.error).length
          return {
            message:
              ok === results.length
                ? t("Installed {{name}} on {{count}} servers", {
                    name: hit.title,
                    count: ok,
                    defaultValue_one: "Installed {{name}} on {{count}} server",
                  })
                : t("Installed {{name}} on {{ok}} of {{count}} servers", { name: hit.title, ok, count: results.length }),
            warning: ok < results.length,
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
            title={title}
            onBackground={() => {
              operation.background(title)
              onOpenChange(false)
            }}
            onBack={() => {
              operation.reset()
              install.reset()
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
            {install.data ? (
              <Results results={install.data} servers={servers} />
            ) : suitable.length === 0 ? (
              <p className="py-4 text-sm text-muted-foreground">{t("None of your servers can run {{name}}.", { name: hit.title })}</p>
            ) : (
              <ul className="-mx-1 grid max-h-80 gap-1 overflow-y-auto px-1">
                {suitable.map((s) => (
                  <li key={key(refOf(s))}>
                    <label className="flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2 ring-1 ring-foreground/8 hover:bg-muted/50">
                      <Checkbox checked={selected.includes(key(refOf(s)))} onCheckedChange={(on) => toggle(key(refOf(s)), on === true)} />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium">{s.name}</span>
                        <span className="block truncate text-xs text-muted-foreground">
                          {serverType(s.type).label} {displayVersion(s.version)} · {s.nodeName}
                        </span>
                      </span>
                    </label>
                  </li>
                ))}
              </ul>
            )}
            {!install.data && chosen.length > 0 && (
              <div className="flex items-center justify-between gap-3 rounded-lg bg-muted/50 px-3 py-2 text-sm">
                <span className="font-medium">{t("Version")}</span>
                {same ? (
                  <VersionMenu
                    project={hit.id}
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
              <DialogClose asChild>
                <Button variant="outline" disabled={install.isPending}>
                  {install.data ? t("Done") : t("Cancel")}
                </Button>
              </DialogClose>
              {!install.data && (
                <Button disabled={selected.length === 0 || install.isPending} onClick={start}>
                  {install.isPending
                    ? t("Installing…")
                    : t("Install on {{count}} servers", { count: selected.length, defaultValue_one: "Install on {{count}} server" })}
                </Button>
              )}
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}

/** What was installed on each server. Servers load new plugins when they restart. */
export function Results({ results, servers }: { results: InstallResult[]; servers: NodeServer[] }) {
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
              <span className="block text-xs text-muted-foreground">{r.error ?? r.installed.map((i) => i.fileName).join(", ")}</span>
            </span>
          </li>
        ))}
      </ul>
      <p className="text-xs text-muted-foreground">{t("Restart the servers to load what was installed.")}</p>
    </div>
  )
}
