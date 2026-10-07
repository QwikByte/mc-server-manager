import { ArchiveIcon, ArrowCounterClockwiseIcon, ClockIcon, DownloadSimpleIcon, PlusIcon, TrashIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, Link } from "@tanstack/react-router"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { Trans } from "react-i18next"
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
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { nodeQuery } from "@/features/nodes/api"
import { guard, useOperation } from "@/features/operations/use-operation"
import { covers } from "@/features/schedules/api"
import { describeSchedule } from "@/features/schedules/describe"
import { type Server, useServer } from "@/features/servers/api"
import { formatBytes, formatDateTime } from "@/lib/format"
import {
  type Backup,
  backupsQuery,
  defaultSelection,
  describeContent,
  downloadUrl,
  jobs,
  nothingSelected,
  pathsError,
  useBackups,
} from "./api"
import { LocationField, SelectionField } from "./backup-fields"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId/backups")

/** The Backups tab of a server. */
export function ServerBackupsPage() {
  const { can } = useAccess()
  const { nodeId, serverId } = route.useParams()
  const { server } = useServer(nodeId, serverId)
  const { data: backups, isPending, error } = useQuery(backupsQuery(nodeId, serverId))
  const { data: jobList = [] } = useQuery({ ...jobs.tasksQuery, enabled: can("backupjobs.view") })
  const create = can("backups.create", nodeId, serverId)
  const covering = jobList.filter((j) => j.enabled && covers(j.targets, nodeId, serverId))

  if (!server || isPending) return <Skeleton className="h-64 rounded-xl" />
  if (error) return <ErrorCallout error={error} />
  const total = backups.reduce((sum, b) => sum + b.size, 0)

  return (
    <Section
      title={t("{{count}} backups", { count: backups.length, defaultValue_one: "{{count}} backup" })}
      description={formatBytes(total)}
      className="mt-0"
      actions={create && <CreateBackupDialog nodeId={nodeId} server={server} />}
    >
      {can("backupjobs.view") && (
        <Callout tone={covering.length > 0 ? "info" : "neutral"} icon={ClockIcon} className="mb-4">
          {covering.length > 0 ? (
            <Trans
              i18nKey="Backed up by <jobs/>."
              components={{
                jobs: (
                  <>
                    {covering.map((j, i) => (
                      <span key={j.id}>
                        {i > 0 && ", "}
                        <Link to="/backups/$jobId" params={{ jobId: j.id }} className="font-medium underline-offset-4 hover:underline">
                          {j.name}
                        </Link>{" "}
                        ({describeSchedule(j.schedule)})
                      </span>
                    ))}
                  </>
                ),
              }}
            />
          ) : (
            <>
              {t("No backup job covers this server.")}
              {can("backupjobs.manage") && (
                <>
                  {" "}
                  <Trans
                    i18nKey="<link>Create a job</link> to back it up on a schedule."
                    components={{ link: <Link to="/backups/new" className="font-medium underline-offset-4 hover:underline" /> }}
                  />
                </>
              )}
            </>
          )}
        </Callout>
      )}
      {backups.length === 0 ? (
        <EmptyState icon={ArchiveIcon} tone="info" title={t("No backups yet")}>
          {create && <CreateBackupDialog nodeId={nodeId} server={server} />}
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
  const { can } = useAccess()
  const { restore, remove } = useBackups(nodeId, server.id)
  const operation = useOperation()
  const created = formatDateTime(backup.createdAt)
  return (
    <li className="flex flex-wrap items-center gap-3 px-4 py-3">
      <IconTile icon={ArchiveIcon} tone="info" size="sm" />
      <div className="min-w-0 flex-1">
        <p className="flex items-center gap-2 text-sm font-semibold">
          <span className="truncate">{backup.label || t("Backup")}</span>
          {backup.jobId && <Pill tone="info">{t("Scheduled")}</Pill>}
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
            <span className="max-sm:sr-only">{t("Download")}</span>
          </a>
        </Button>
        {can("backups.restore", nodeId, server.id) && (
          <ConfirmDialog
            trigger={
              <Button size="sm" variant="outline" disabled={restore.isPending}>
                <ArrowCounterClockwiseIcon />
                <span className="max-sm:sr-only">{t("Restore")}</span>
              </Button>
            }
            title={t("Restore the backup of {{time}}?", { time: created })}
            description={[
              t("This replaces {{content}} of {{name}} with the backed up state; what was added since is removed.", {
                content: backup.paths.includes(".") ? t("everything") : describeContent(backup.paths),
                name: server.name,
              }),
              server.state !== "stopped" && t("The server stops while this happens and starts again afterwards."),
            ]
              .filter(Boolean)
              .join(" ")}
            action={t("Restore")}
            destructive
            onConfirm={() =>
              operation.run((onStart) => restore.mutateAsync({ id: backup.id, onStart }), {
                title: t("Restoring {{name}}…", { name: server.name }),
                notify: true,
                done: ({ warning }) => ({
                  message: t("Restored the backup of {{time}}", { time: created }),
                  description: warning,
                  warning: !!warning,
                }),
              })
            }
          />
        )}
        {can("backups.delete", nodeId, server.id) && (
          <ConfirmDialog
            trigger={
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label={t("Delete the backup of {{time}}", { time: created })}
                title={t("Delete")}
                disabled={remove.isPending}
                className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
              >
                <TrashIcon />
              </Button>
            }
            title={t("Delete the backup of {{time}}?", { time: created })}
            description={t("The backup is deleted from the node. This can't be undone.")}
            action={t("Delete backup")}
            destructive
            onConfirm={() =>
              remove.mutate(backup.id, { onSuccess: () => toast.success(t("Deleted the backup")), onError: (e) => toast.error(e.message) })
            }
          />
        )}
      </div>
    </li>
  )
}

/**
 * Backs up a server by hand. Once the master backs it up, the dialog closes and a notification follows the backup, as
 * large worlds take a while; a request that fails right away shows its error in the dialog.
 */
function CreateBackupDialog({ nodeId, server }: { nodeId: string; server: Server }) {
  const [open, setOpen] = useState(false)
  const [label, setLabel] = useState("")
  const [selection, setSelection] = useState(defaultSelection)
  const [location, setLocation] = useState("")
  const { data: node } = useQuery(nodeQuery(nodeId))
  const { create } = useBackups(nodeId, server.id)
  const operation = useOperation()

  const close = () => {
    setOpen(false)
    setLabel("")
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    const title = t("Backing up {{name}}…", { name: server.name })
    operation.run(
      (onStart) =>
        create.mutateAsync({
          label: label.trim(),
          selection,
          location,
          onStart: (op) => {
            onStart(op)
            operation.background(title, op.id)
            close()
          },
        }),
      {
        title,
        done: (b) => ({ message: t("Backed up {{name}} ({{size}})", { name: server.name, size: formatBytes(b.size) }) }),
        then: close,
      },
    )
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (next) create.reset()
      }}
    >
      <DialogTrigger asChild>
        <Button disabled={create.isPending}>
          <PlusIcon />
          {create.isPending ? t("Backing up…") : t("Back up now")}
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl" {...guard(create.isPending)}>
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{t("Back up {{name}}", { name: server.name })}</DialogTitle>
            <DialogDescription>
              {server.state === "stopped"
                ? t("The backup is kept on the node.")
                : t("The server saves its worlds first and keeps running. The backup is kept on the node.")}
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="backup-label">{t("Label")}</FieldLabel>
              <Input
                id="backup-label"
                maxLength={64}
                placeholder={t("Before the update")}
                value={label}
                onChange={(e) => setLabel(e.target.value)}
              />
            </Field>
            <SelectionField value={selection} server={{ nodeId, serverId: server.id }} onChange={setSelection} />
            <LocationField locations={node?.info?.storage.map((l) => l.name) ?? []} value={location} onChange={setLocation} />
          </FieldGroup>
          {create.error && <FieldError>{create.error.message}</FieldError>}
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline" disabled={create.isPending}>
                {t("Cancel")}
              </Button>
            </DialogClose>
            <Button type="submit" disabled={nothingSelected(selection) || !!pathsError(selection.paths) || create.isPending}>
              {create.isPending ? t("Backing up…") : t("Back up")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
