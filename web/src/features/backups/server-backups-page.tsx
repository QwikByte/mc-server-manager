import { ArchiveIcon, ArrowCounterClockwiseIcon, ClockIcon, DownloadSimpleIcon, PlusIcon, TrashIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, Link } from "@tanstack/react-router"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { Callout, ErrorCallout } from "@/components/callout"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { EmptyState } from "@/components/empty-state"
import { IconTile } from "@/components/icon-tile"
import { Section } from "@/components/section"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
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
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { nodeQuery } from "@/features/nodes/api"
import { covers } from "@/features/schedules/api"
import { describeSchedule } from "@/features/schedules/describe"
import { type Server, useServer } from "@/features/servers/api"
import { formatBytes, formatDateTime } from "@/lib/format"
import { type Backup, backupsQuery, defaultSelection, describeContent, downloadUrl, jobs, nothingSelected, useBackups } from "./api"
import { LocationField, SelectionField } from "./backup-fields"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId/backups")

/** The Backups tab of a server. */
export function ServerBackupsPage() {
  const { nodeId, serverId } = route.useParams()
  const { server } = useServer(nodeId, serverId)
  const { data: backups, isPending, error } = useQuery(backupsQuery(nodeId, serverId))
  const { data: jobList = [] } = useQuery(jobs.tasksQuery)
  const covering = jobList.filter((j) => j.enabled && covers(j.targets, nodeId, serverId))

  if (!server || isPending) return <Skeleton className="h-64 rounded-xl" />
  if (error) return <ErrorCallout error={error} />
  const total = backups.reduce((sum, b) => sum + b.size, 0)

  return (
    <Section
      title={`${backups.length} ${backups.length === 1 ? "backup" : "backups"}`}
      description={`${formatBytes(total)} on the node. Restoring replaces what a backup contains.`}
      className="mt-0"
      actions={<CreateBackupDialog nodeId={nodeId} server={server} />}
    >
      <Callout tone={covering.length > 0 ? "info" : "neutral"} icon={ClockIcon} className="mb-4">
        {covering.length > 0 ? (
          <>
            Backed up by{" "}
            {covering.map((j, i) => (
              <span key={j.id}>
                {i > 0 && ", "}
                <Link to="/backups/$jobId" params={{ jobId: j.id }} className="font-medium underline-offset-4 hover:underline">
                  {j.name}
                </Link>{" "}
                ({describeSchedule(j.schedule).toLowerCase()})
              </span>
            ))}
            .
          </>
        ) : (
          <>
            No backup job covers this server.{" "}
            <Link to="/backups/new" className="font-medium underline-offset-4 hover:underline">
              Create a job
            </Link>{" "}
            to back it up on a schedule.
          </>
        )}
      </Callout>
      {backups.length === 0 ? (
        <EmptyState icon={ArchiveIcon} tone="info" title="No backups yet" description="Back up the server now, e.g. before an update.">
          <CreateBackupDialog nodeId={nodeId} server={server} />
        </EmptyState>
      ) : (
        <ul className="surface divide-y rounded-xl">
          {backups.map((b) => (
            <BackupRow key={b.id} nodeId={nodeId} server={server} backup={b} />
          ))}
        </ul>
      )}
    </Section>
  )
}

function BackupRow({ nodeId, server, backup }: { nodeId: string; server: Server; backup: Backup }) {
  const { restore, remove } = useBackups(nodeId, server.id)
  const created = formatDateTime(backup.createdAt)
  return (
    <li className="flex flex-wrap items-center gap-3 px-4 py-3">
      <IconTile icon={ArchiveIcon} tone="info" size="sm" />
      <div className="min-w-0 flex-1">
        <p className="flex items-center gap-2 text-sm font-semibold">
          <span className="truncate">{backup.label || "Backup"}</span>
          {backup.jobId && <Pill tone="info">Scheduled</Pill>}
        </p>
        <p className="truncate text-xs text-muted-foreground">
          {created} · {formatBytes(backup.size)} · {describeContent(backup.paths)}
          {backup.location !== "default" && ` · ${backup.location}`}
        </p>
      </div>
      <div className="flex items-center gap-1.5">
        <Button asChild size="sm" variant="outline">
          <a href={downloadUrl(nodeId, server.id, backup.id)} download>
            <DownloadSimpleIcon />
            <span className="max-sm:sr-only">Download</span>
          </a>
        </Button>
        <ConfirmDialog
          trigger={
            <Button size="sm" variant="outline" disabled={restore.isPending}>
              <ArrowCounterClockwiseIcon />
              <span className="max-sm:sr-only">Restore</span>
            </Button>
          }
          title={`Restore the backup of ${created}?`}
          description={`This replaces ${describeContent(backup.paths).toLowerCase()} of ${server.name} with the backed up state; what was added since is removed.${
            server.state === "stopped" ? "" : " The server stops meanwhile and starts again."
          }`}
          action="Restore"
          destructive
          onConfirm={() =>
            toast.promise(restore.mutateAsync(backup.id), {
              loading: `Restoring ${server.name}…`,
              success: `Restored the backup of ${created}`,
              error: (e: Error) => e.message,
            })
          }
        />
        <ConfirmDialog
          trigger={
            <Button
              size="icon-sm"
              variant="ghost"
              aria-label={`Delete the backup of ${created}`}
              title="Delete"
              disabled={remove.isPending}
              className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
            >
              <TrashIcon />
            </Button>
          }
          title={`Delete the backup of ${created}?`}
          description="The backup is deleted from the node. This can't be undone."
          action="Delete backup"
          destructive
          onConfirm={() =>
            remove.mutate(backup.id, { onSuccess: () => toast.success("Deleted the backup"), onError: (e) => toast.error(e.message) })
          }
        />
      </div>
    </li>
  )
}

/** Backs up a server by hand. The dialog closes right away, as large worlds take a while. */
function CreateBackupDialog({ nodeId, server }: { nodeId: string; server: Server }) {
  const [open, setOpen] = useState(false)
  const [label, setLabel] = useState("")
  const [selection, setSelection] = useState(defaultSelection)
  const [location, setLocation] = useState("")
  const { data: node } = useQuery(nodeQuery(nodeId))
  const { create } = useBackups(nodeId, server.id)

  function submit(event: FormEvent) {
    event.preventDefault()
    setOpen(false)
    toast.promise(create.mutateAsync({ label: label.trim(), selection, location }), {
      loading: `Backing up ${server.name}…`,
      success: (b) => `Backed up ${server.name} (${formatBytes(b.size)})`,
      error: (e: Error) => e.message,
    })
    setLabel("")
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button disabled={create.isPending}>
          <PlusIcon />
          {create.isPending ? "Backing up…" : "Back up now"}
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-2xl">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>Back up {server.name}</DialogTitle>
            <DialogDescription>
              {server.state === "stopped"
                ? "The backup is kept on the node."
                : "The server saves its worlds first and keeps running. The backup is kept on the node."}
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="backup-label">Label</FieldLabel>
              <Input
                id="backup-label"
                maxLength={64}
                placeholder="Before the update to 1.21.5"
                value={label}
                onChange={(e) => setLabel(e.target.value)}
              />
            </Field>
            <SelectionField value={selection} onChange={setSelection} />
            <LocationField locations={node?.info?.storage.map((l) => l.name) ?? []} value={location} onChange={setLocation} />
          </FieldGroup>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">Cancel</Button>
            </DialogClose>
            <Button type="submit" disabled={nothingSelected(selection)}>
              Back up
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
