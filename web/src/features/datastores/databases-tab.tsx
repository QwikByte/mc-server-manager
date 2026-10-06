import {
  ArchiveIcon,
  ArrowCounterClockwiseIcon,
  DatabaseIcon,
  DownloadSimpleIcon,
  GearIcon,
  HardDrivesIcon,
  KeyIcon,
  MemoryIcon,
  PlayIcon,
  PlusIcon,
  StopIcon,
  TrashIcon,
  WarningCircleIcon,
} from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, Link } from "@tanstack/react-router"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { Callout, ErrorCallout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { CopyField } from "@/components/copy-field"
import { EmptyState } from "@/components/empty-state"
import { IconTile } from "@/components/icon-tile"
import { Section } from "@/components/section"
import { Pill, StatusBadge } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { useOperation } from "@/features/operations/use-operation"
import { formatBytes, formatDateTime, formatMegabytes } from "@/lib/format"
import {
  type Database,
  type DatabaseUse,
  type Datastore,
  downloadUrl,
  type Dump,
  dumpsQuery,
  fields,
  networkDatastoresQuery,
  placeholderOf,
  useDatastore,
} from "./api"
import { ChangeDatastoreDialog, CreateDatastoreDialog, DeleteDatastoreDialog } from "./datastore-dialogs"
import { engines, states } from "./labels"

const route = getRouteApi("/_app/networks/$networkId/databases")

const exampleDatabase = "luckperms"

/** The Databases tab of a network: its datastores, their databases and dumps. */
export function DatabasesTab() {
  const { networkId } = route.useParams()
  const { can } = useAccess()
  const { data, isPending, error } = useQuery(networkDatastoresQuery(networkId))
  if (isPending) return <Skeleton className="h-64 rounded-xl" />
  if (error) return <ErrorCallout error={error} />
  return (
    <Section
      title={t("Datastores")}
      description={t(
        "MariaDB and PostgreSQL servers that only the servers of this network reach. File sets put the credentials of their databases into the configuration of plugins.",
      )}
      className="mt-0"
      actions={can("datastores.manage") && data.datastores.length > 0 && <CreateDatastoreDialog networkId={networkId} />}
    >
      {data.datastores.length === 0 ? (
        <EmptyState
          icon={DatabaseIcon}
          tone="info"
          title={t("No datastores yet")}
          description={t("Add one for plugins that share their data across the network, e.g. LuckPerms or CoreProtect.")}
        >
          {can("datastores.manage") && <CreateDatastoreDialog networkId={networkId} />}
        </EmptyState>
      ) : (
        <div className="space-y-6">
          {data.datastores.map((ds) => (
            <DatastoreCard key={ds.id} datastore={ds} uses={data.uses.filter((u) => u.database.startsWith(`${ds.name}.`))} />
          ))}
        </div>
      )}
    </Section>
  )
}

function DatastoreCard({ datastore: ds, uses }: { datastore: Datastore; uses: DatabaseUse[] }) {
  const { can } = useAccess()
  const manage = can("datastores.manage")
  const { power, addDatabase, update } = useDatastore(ds.id)
  const run = (action: "start" | "stop") =>
    power.mutate(action, {
      onSuccess: () => toast.success(action === "start" ? t("Started {{name}}", { name: ds.name }) : t("Stopped {{name}}", { name: ds.name })),
      onError: (e) => toast.error(e.message),
    })

  return (
    <article className="surface rounded-xl">
      <header className="flex flex-wrap items-center gap-3 border-b px-4 py-3">
        <IconTile icon={DatabaseIcon} tone="violet" size="sm" />
        <div className="min-w-0 flex-1 basis-56">
          <p className="flex flex-wrap items-center gap-2 font-semibold">
            <span className="font-mono">{ds.name}</span>
            <StatusBadge status={states[ds.state]} />
          </p>
          <p className="mt-1 flex flex-wrap gap-1.5">
            <Chip icon={DatabaseIcon}>
              {engines[ds.engine].label} {ds.version}
            </Chip>
            <Chip icon={HardDrivesIcon}>{ds.nodeName}</Chip>
            <Chip icon={MemoryIcon}>{formatMegabytes(ds.memoryMb)}</Chip>
            <Chip icon={ArchiveIcon}>{formatBytes(ds.size)}</Chip>
          </p>
        </div>
        {manage && (
          <div className="flex items-center gap-1.5">
            {ds.state === "stopped" ? (
              <Button size="sm" variant="outline" disabled={power.isPending} onClick={() => run("start")}>
                <PlayIcon />
                {t("Start")}
              </Button>
            ) : (
              <ConfirmDialog
                trigger={
                  <Button size="sm" variant="outline" disabled={power.isPending || ds.state === "unknown"}>
                    <StopIcon />
                    {t("Stop")}
                  </Button>
                }
                title={t("Stop {{name}}?", { name: ds.name })}
                description={t("The plugins that use its databases lose them until it starts again.")}
                action={t("Stop")}
                destructive
                onConfirm={() => run("stop")}
              />
            )}
            <ChangeDatastoreDialog
              datastore={ds}
              trigger={
                <Button size="icon-sm" variant="ghost" aria-label={t("Change {{name}}", { name: ds.name })} title={t("Change")}>
                  <GearIcon />
                </Button>
              }
            />
            <DeleteDatastoreDialog
              datastore={ds}
              trigger={
                <Button
                  size="icon-sm"
                  variant="ghost"
                  aria-label={t("Delete {{name}}", { name: ds.name })}
                  title={t("Delete")}
                  className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                >
                  <TrashIcon />
                </Button>
              }
            />
          </div>
        )}
      </header>
      <div className="space-y-4 p-4">
        {ds.problem && (
          <Callout tone="warning" icon={WarningCircleIcon} role="alert">
            {ds.problem}
          </Callout>
        )}
        {ds.state === "unhealthy" && (
          <Callout tone="destructive" icon={WarningCircleIcon} role="alert">
            {t("Its health check fails. The log of its container on {{node}} tells why.", { node: ds.nodeName })}
          </Callout>
        )}
        {ds.missing.length > 0 && (
          <Callout tone="warning" icon={WarningCircleIcon} title={t("Databases are missing")}>
            {t("The datastore lacks {{names}}, e.g. as its data was replaced.", { names: ds.missing.join(", ") })}
            {manage && (
              <div className="mt-2 flex flex-wrap gap-2">
                {ds.missing.map((name) => (
                  <Button key={name} size="xs" variant="outline" disabled={addDatabase.isPending} onClick={() => addDatabase.mutate(name)}>
                    {t("Create {{name}} again", { name })}
                  </Button>
                ))}
              </div>
            )}
          </Callout>
        )}
        {ds.previous && (
          <Callout tone="info" title={t("The data of version {{version}} is kept", { version: ds.previous })}>
            {t("Remove it once the new version works, to free its space.")}
            {manage && (
              <div className="mt-2">
                <ConfirmDialog
                  trigger={
                    <Button size="xs" variant="outline" disabled={update.isPending}>
                      {t("Remove the data of {{version}}", { version: ds.previous })}
                    </Button>
                  }
                  title={t("Remove the data of {{version}}?", { version: ds.previous })}
                  description={t("You can't go back to that version afterwards. The datastore restarts.")}
                  action={t("Remove")}
                  destructive
                  onConfirm={() => update.mutate({ removePrevious: true }, { onError: (e) => toast.error(e.message) })}
                />
              </div>
            )}
          </Callout>
        )}
        <Databases datastore={ds} uses={uses} />
        <Dumps datastore={ds} />
      </div>
    </article>
  )
}

function Databases({ datastore: ds, uses }: { datastore: Datastore; uses: DatabaseUse[] }) {
  const { can } = useAccess()
  const [name, setName] = useState("")
  const { addDatabase } = useDatastore(ds.id)

  function submit(event: FormEvent) {
    event.preventDefault()
    addDatabase.mutate(name.trim(), {
      onSuccess: (db) => {
        toast.success(t("Created the database {{name}}", { name: db.name }))
        setName("")
      },
    })
  }

  return (
    <section aria-label={t("Databases")} className="space-y-2">
      <h3 className="text-sm font-semibold">{t("Databases")}</h3>
      {ds.databases.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("No databases yet. Each plugin gets one, with a user of the same name.")}</p>
      ) : (
        <ul className="divide-y rounded-lg ring-1 ring-foreground/8">
          {ds.databases.map((db) => (
            <DatabaseRow key={db.name} datastore={ds} database={db} uses={uses.filter((u) => u.database === `${ds.name}.${db.name}`)} />
          ))}
        </ul>
      )}
      {can("datastores.manage") && (
        <form onSubmit={submit} className="flex flex-wrap items-start gap-2">
          <Input
            aria-label={t("Name of the new database")}
            required
            maxLength={32}
            pattern="[a-z][a-z0-9_]*"
            placeholder={exampleDatabase}
            className="w-56 font-mono"
            value={name}
            onChange={(e) => setName(e.target.value.toLowerCase())}
          />
          <Button type="submit" size="sm" variant="outline" disabled={addDatabase.isPending || ds.state !== "running"}>
            <PlusIcon />
            {t("Add database")}
          </Button>
          {addDatabase.error && <p className="w-full text-sm text-destructive">{addDatabase.error.message}</p>}
        </form>
      )}
    </section>
  )
}

function DatabaseRow({ datastore: ds, database: db, uses }: { datastore: Datastore; database: Database; uses: DatabaseUse[] }) {
  const { can } = useAccess()
  const [open, setOpen] = useState(false)
  const { dropDatabase, rotate } = useDatastore(ds.id)
  const operation = useOperation()
  const servers = [...new Map(uses.map((u) => [`${u.nodeId}/${u.serverId}`, u])).values()]

  return (
    <li className="space-y-3 px-3 py-2.5">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-mono text-sm font-semibold">{db.name}</span>
        {servers.length > 0 ? (
          <span className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
            {t("Used by")}
            {servers.map((u, i) => (
              <span key={`${u.nodeId}/${u.serverId}`}>
                <Link
                  to="/nodes/$nodeId/servers/$serverId"
                  params={{ nodeId: u.nodeId, serverId: u.serverId }}
                  className="font-medium text-foreground underline-offset-4 hover:underline"
                >
                  {u.name ?? u.serverId}
                </Link>
                {i < servers.length - 1 && ","}
              </span>
            ))}
          </span>
        ) : (
          <Pill tone="neutral">{t("No file set uses it")}</Pill>
        )}
        <div className="ml-auto flex items-center gap-1.5">
          <Button size="sm" variant="ghost" aria-expanded={open} onClick={() => setOpen(!open)}>
            {open ? t("Hide placeholders") : t("Placeholders")}
          </Button>
          {can("datastores.manage") && (
            <>
              <ConfirmDialog
                trigger={
                  <Button size="sm" variant="outline" disabled={rotate.isPending}>
                    <KeyIcon />
                    <span className="max-sm:sr-only">{t("New password")}</span>
                  </Button>
                }
                title={t("Give {{name}} a new password?", { name: db.name })}
                description={t(
                  "The file sets that use it are applied again, and their running servers restart to load it. Servers that it reaches otherwise lose access.",
                )}
                action={t("New password")}
                onConfirm={() =>
                  operation.run((onStart) => rotate.mutateAsync({ name: db.name, onStart }), {
                    title: t("Rotating the password of {{name}}…", { name: db.name }),
                    notify: true,
                    done: ({ results }) => ({
                      message: t("{{name}} has a new password", { name: db.name }),
                      description: t("{{count}} servers got it", { count: results.filter((r) => !r.error).length, defaultValue_one: "{{count}} server got it" }),
                      warning: results.some((r) => r.error),
                    }),
                  })
                }
              />
              <ConfirmDialog
                trigger={
                  <Button
                    size="icon-sm"
                    variant="ghost"
                    aria-label={t("Drop {{name}}", { name: db.name })}
                    title={t("Drop")}
                    disabled={dropDatabase.isPending}
                    className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                  >
                    <TrashIcon />
                  </Button>
                }
                title={t("Drop the database {{name}}?", { name: db.name })}
                description={t("Its user and all its data are deleted; backups keep theirs. This can't be undone.")}
                action={t("Drop database")}
                destructive
                onConfirm={() =>
                  dropDatabase.mutate(db.name, {
                    onSuccess: () => toast.success(t("Dropped the database {{name}}", { name: db.name })),
                    onError: (e) => toast.error(e.message),
                  })
                }
              />
            </>
          )}
        </div>
      </div>
      {open && (
        <div className="grid gap-2 sm:grid-cols-2">
          {fields.map((f) => (
            <CopyField key={f} value={placeholderOf(ds.name, db.name, f)} label={f} />
          ))}
        </div>
      )}
    </li>
  )
}

function Dumps({ datastore: ds }: { datastore: Datastore }) {
  const { can } = useAccess()
  const { data: dumps, isPending, error } = useQuery(dumpsQuery(ds.id))
  const { dump } = useDatastore(ds.id)
  const operation = useOperation()

  const create = () => {
    const title = t("Backing up {{name}}…", { name: ds.name })
    operation.run((onStart) => dump.mutateAsync({ label: "", databases: [], onStart }), {
      title,
      notify: true,
      done: (d) => ({ message: t("Backed up {{name}} ({{size}})", { name: ds.name, size: formatBytes(d.size) }) }),
    })
  }

  return (
    <section aria-label={t("Backups")} className="space-y-2">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-sm font-semibold">{t("Backups")}</h3>
        {can("datastores.manage") && (
          <Button size="sm" variant="outline" disabled={dump.isPending || ds.state !== "running" || ds.databases.length === 0} onClick={create}>
            <ArchiveIcon />
            {t("Back up now")}
          </Button>
        )}
      </div>
      {isPending ? (
        <Skeleton className="h-12 rounded-lg" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : dumps.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("No backups yet. Backup jobs can back the datastore up on a schedule.")}</p>
      ) : (
        <ul className="divide-y rounded-lg ring-1 ring-foreground/8">
          {dumps.map((d) => (
            <DumpRow key={d.id} datastore={ds} dump={d} />
          ))}
        </ul>
      )}
    </section>
  )
}

function DumpRow({ datastore: ds, dump: d }: { datastore: Datastore; dump: Dump }) {
  const { can } = useAccess()
  const { restore, removeDump } = useDatastore(ds.id)
  const operation = useOperation()
  const created = formatDateTime(d.createdAt)
  return (
    <li className="flex flex-wrap items-center gap-3 px-3 py-2">
      <div className="min-w-0 flex-1">
        <p className="flex items-center gap-2 text-sm font-medium">
          <span className="truncate">{d.label || created}</span>
          {d.jobId && <Pill tone="info">{t("Scheduled")}</Pill>}
        </p>
        <p className="truncate text-xs text-muted-foreground">
          {d.label && `${created} · `}
          {formatBytes(d.size)} · {d.databases.join(", ")}
        </p>
      </div>
      {can("datastores.manage") && (
        <div className="flex items-center gap-1.5">
          <Button asChild size="sm" variant="outline">
            <a href={downloadUrl(ds.id, d.id)} download>
              <DownloadSimpleIcon />
              <span className="max-sm:sr-only">{t("Download")}</span>
            </a>
          </Button>
          <ConfirmDialog
            trigger={
              <Button size="sm" variant="outline" disabled={restore.isPending}>
                <ArrowCounterClockwiseIcon />
                <span className="max-sm:sr-only">{t("Restore")}</span>
              </Button>
            }
            title={t("Restore the backup of {{time}}?", { time: created })}
            description={t(
              "This replaces {{names}} with the backed up state; what was added since is lost. The running servers whose file sets use them stop meanwhile and start again.",
              { names: d.databases.join(", ") },
            )}
            action={t("Restore")}
            destructive
            onConfirm={() =>
              operation.run((onStart) => restore.mutateAsync({ dump: d.id, databases: [], onStart }), {
                title: t("Restoring {{name}}…", { name: ds.name }),
                notify: true,
                done: () => ({ message: t("Restored the backup of {{time}}", { time: created }) }),
              })
            }
          />
          <ConfirmDialog
            trigger={
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label={t("Delete the backup of {{time}}", { time: created })}
                title={t("Delete")}
                disabled={removeDump.isPending}
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
              removeDump.mutate(d.id, { onSuccess: () => toast.success(t("Deleted the backup")), onError: (e) => toast.error(e.message) })
            }
          />
        </div>
      )}
    </li>
  )
}
