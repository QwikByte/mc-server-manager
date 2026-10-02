import { CheckCircleIcon, DownloadSimpleIcon, WarningCircleIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
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
import { type InstallResult, type SearchHit, supports, useInstallPlugins } from "./api"
import { PluginIcon } from "./plugin-icon"

/** Installs a plugin or mod on any number of servers it runs on. */
export function InstallDialog({ hit }: { hit: SearchHit }) {
  const [open, setOpen] = useState(false)
  const [selected, setSelected] = useState<string[]>([])
  const { data: servers = [] } = useQuery({ ...allServersQuery, enabled: open })
  const install = useInstallPlugins()
  const { can } = useAccess()
  const suitable = servers.filter((s) => supports(hit.loaders, s.type) && can("plugins.manage", s.nodeId, s.id))
  const toggle = (k: string, on: boolean) => setSelected((list) => (on ? [...list, k] : list.filter((s) => s !== k)))

  function onOpenChange(next: boolean) {
    setOpen(next)
    if (!next) {
      install.reset()
      setSelected([])
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline">
          <DownloadSimpleIcon />
          Install
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader className="flex-row items-center gap-3">
          <PluginIcon src={hit.icon} />
          <div className="min-w-0 space-y-1 text-left">
            <DialogTitle>Install {hit.title}</DialogTitle>
            <DialogDescription>The newest release that suits each server is installed, with what it requires.</DialogDescription>
          </div>
        </DialogHeader>
        {install.data ? (
          <Results results={install.data} servers={servers} />
        ) : suitable.length === 0 ? (
          <p className="py-4 text-sm text-muted-foreground">None of your servers can run {hit.title}.</p>
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
        {install.error && <FieldError>{install.error.message}</FieldError>}
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">{install.data ? "Done" : "Cancel"}</Button>
          </DialogClose>
          {!install.data && (
            <Button
              disabled={selected.length === 0 || install.isPending}
              onClick={() =>
                install.mutate({ projects: [hit.id], servers: suitable.filter((s) => selected.includes(key(refOf(s)))).map(refOf) })
              }
            >
              {install.isPending ? "Installing…" : `Install on ${selected.length || ""} ${selected.length === 1 ? "server" : "servers"}`}
            </Button>
          )}
        </DialogFooter>
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
      <p className="text-xs text-muted-foreground">Restart the servers to load what was installed.</p>
    </div>
  )
}
