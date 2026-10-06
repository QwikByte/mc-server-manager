import { ArrowUpIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { Fragment, type ReactNode, useEffect, useState } from "react"
import { ErrorCallout } from "@/components/callout"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldError, FieldLabel } from "@/components/ui/field"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { allServersQuery } from "@/features/servers/api"
import { ApiError } from "@/lib/api"
import { formatBytes } from "@/lib/format"
import { filesQuery, join, type ServerFiles } from "./api"
import { FileTypeIcon } from "./file-icon"

/** A file or folder ticked in a PathPicker. */
export interface Picked {
  path: string
  directory: boolean
  size: number
}

const crumb = "rounded px-1 py-0.5 transition-colors hover:bg-muted hover:text-foreground"

/**
 * Browses the folders of a server. With picked, files and folders can be ticked, and what is in
 * a ticked folder counts as ticked; without, only folders show and the one shown is the choice.
 */
export function PathPicker({
  server,
  folder,
  onFolder,
  picked,
  onPick,
}: {
  server: ServerFiles
  folder: string
  onFolder: (path: string) => void
  picked?: Picked[]
  onPick?: (picked: Picked[]) => void
}) {
  const { data, isPending, error } = useQuery(filesQuery(server, folder))
  const segments = folder ? folder.split("/") : []
  // A server without the folder, e.g. one a set's file is in, shows the nearest one it has.
  const missing = error instanceof ApiError && error.status === 404 && folder !== ""
  useEffect(() => {
    if (missing) onFolder(segments.slice(0, -1).join("/"))
  })
  const entries = (data?.files ?? [])
    .filter((e) => picked || e.directory)
    .sort((a, b) => Number(b.directory) - Number(a.directory) || a.name.localeCompare(b.name))
  const inPicked = (path: string) => picked?.some((p) => path.startsWith(`${p.path}/`))
  const toggle = (item: Picked) =>
    onPick?.(picked?.some((p) => p.path === item.path) ? picked.filter((p) => p.path !== item.path) : [...(picked ?? []), item])

  return (
    <div className="overflow-hidden rounded-lg ring-1 ring-foreground/10">
      <nav aria-label={t("Folder")} className="flex flex-wrap items-center gap-0.5 border-b bg-muted/40 px-2 py-1.5 font-mono text-xs text-muted-foreground">
        <button type="button" className={crumb} onClick={() => onFolder("")}>
          {t("Server folder")}
        </button>
        {segments.map((name, i) => (
          <Fragment key={segments.slice(0, i + 1).join("/")}>
            <span aria-hidden>/</span>
            <button type="button" className={i === segments.length - 1 ? `${crumb} text-foreground` : crumb} onClick={() => onFolder(segments.slice(0, i + 1).join("/"))}>
              {name}
            </button>
          </Fragment>
        ))}
      </nav>
      <div className="max-h-72 min-h-40 overflow-y-auto">
        {isPending || missing ? (
          <Skeleton className="m-2 h-32 rounded-md" />
        ) : error ? (
          <div className="p-2">
            <ErrorCallout error={error} />
          </div>
        ) : (
          <ul className="divide-y text-sm">
            {folder && (
              <li>
                <button
                  type="button"
                  onClick={() => onFolder(segments.slice(0, -1).join("/"))}
                  className="flex w-full items-center gap-2 px-3 py-1.5 text-left text-muted-foreground hover:bg-muted"
                >
                  <ArrowUpIcon className="size-4" />
                  {t("Up")}
                </button>
              </li>
            )}
            {entries.length === 0 && (
              <li className="px-3 py-6 text-center text-muted-foreground">{picked ? t("This folder is empty.") : t("This folder has no folders.")}</li>
            )}
            {entries.map((e) => {
              const item = { path: join(folder, e.name), directory: e.directory, size: e.size }
              const implied = inPicked(item.path)
              const checked = implied || picked?.some((p) => p.path === item.path)
              return (
                <li key={e.name} className="flex items-center gap-2 px-3 py-1 hover:bg-muted/60">
                  {picked && (
                    <Checkbox
                      checked={checked}
                      disabled={implied}
                      onCheckedChange={() => toggle(item)}
                      aria-label={t("Choose {{name}}", { name: e.name })}
                    />
                  )}
                  <button
                    type="button"
                    onClick={() => (e.directory ? onFolder(item.path) : !implied && toggle(item))}
                    className="flex min-w-0 flex-1 items-center gap-2 py-0.5 text-left"
                  >
                    <FileTypeIcon entry={e} />
                    <span className="min-w-0 flex-1 truncate">{e.name}</span>
                    {!e.directory && <span className="shrink-0 text-xs text-muted-foreground tabular-nums">{formatBytes(e.size)}</span>}
                  </button>
                </li>
              )
            })}
          </ul>
        )}
        {data?.truncated && <p className="px-3 py-2 text-xs text-muted-foreground">{t("The folder has more entries than shown.")}</p>}
      </div>
    </div>
  )
}

const keyOf = (s: ServerFiles) => `${s.nodeId}/${s.serverId}`

/** Chooses one of the servers whose files the user may read. */
export function ServerSelect({ id, value, onChange }: { id?: string; value?: ServerFiles; onChange: (server: ServerFiles) => void }) {
  const { can } = useAccess()
  const { data: servers } = useQuery(allServersQuery)
  const readable = (servers ?? []).filter((s) => can("files.read", s.nodeId, s.id))
  return (
    <Select
      value={value ? keyOf(value) : ""}
      onValueChange={(key) => {
        const s = readable.find((r) => keyOf({ nodeId: r.nodeId, serverId: r.id }) === key)
        if (s) onChange({ nodeId: s.nodeId, serverId: s.id })
      }}
    >
      <SelectTrigger id={id} className="w-full">
        <SelectValue placeholder={t("Choose a server")} />
      </SelectTrigger>
      <SelectContent>
        {readable.map((s) => (
          <SelectItem key={keyOf({ nodeId: s.nodeId, serverId: s.id })} value={keyOf({ nodeId: s.nodeId, serverId: s.id })}>
            {s.name} <span className="text-muted-foreground">· {s.nodeName}</span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

/** Picks files and folders of a server, which may be chosen first, in a dialog. */
export function PickDialog({
  title,
  description,
  server: fixed,
  folder: initial = "",
  action,
  onClose,
  onPick,
}: {
  title: string
  description?: ReactNode
  server?: ServerFiles
  folder?: string
  action: string
  onClose: () => void
  /** Takes what was picked; the dialog closes once it is done, or shows why it failed. */
  onPick: (server: ServerFiles, picked: Picked[]) => Promise<void> | void
}) {
  const [server, setServer] = useState(fixed)
  const [folder, setFolder] = useState(initial)
  const [picked, setPicked] = useState<Picked[]>([])
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string>()

  async function pick() {
    if (!server) return
    setPending(true)
    setError(undefined)
    try {
      await onPick(server, picked)
      onClose()
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setPending(false)
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !pending && onClose()}>
      <DialogContent className="sm:max-w-xl">
        <div className="grid gap-5">
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            {description && <DialogDescription>{description}</DialogDescription>}
          </DialogHeader>
          {!fixed && (
            <Field>
              <FieldLabel htmlFor="pick-server">{t("Server")}</FieldLabel>
              <ServerSelect
                id="pick-server"
                value={server}
                onChange={(s) => {
                  setServer(s)
                  setPicked([])
                }}
              />
            </Field>
          )}
          {server && <PathPicker server={server} folder={folder} onFolder={setFolder} picked={picked} onPick={setPicked} />}
          {error && <FieldError>{error}</FieldError>}
          <DialogFooter className="items-center">
            <p className="mr-auto text-sm text-muted-foreground">
              {t("{{count}} chosen", { count: picked.length })}
            </p>
            <DialogClose asChild>
              <Button variant="outline" disabled={pending}>
                {t("Cancel")}
              </Button>
            </DialogClose>
            <Button type="button" disabled={pending || picked.length === 0} onClick={pick}>
              {action}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  )
}
