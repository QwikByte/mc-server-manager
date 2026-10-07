import { ArrowCounterClockwiseIcon, CaretDownIcon, CaretRightIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { useState } from "react"
import { ErrorCallout } from "@/components/callout"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { formatDateTime } from "@/lib/format"
import { type FileSet, type SetFile, versionQuery } from "./api"
import { DiffView } from "./diff-view"

/** The kept versions of a set, what each changed, and restoring one into the editor. */
export function HistoryTab({
  set,
  dirty,
  editable,
  onRestore,
}: {
  set: FileSet
  dirty: boolean
  editable: boolean
  onRestore: (files: SetFile[]) => void
}) {
  const [open, setOpen] = useState<number>()
  const oldest = set.versions.at(-1)?.version ?? set.version
  return (
    <ol className="surface divide-y overflow-hidden rounded-xl">
      {set.versions.map((v) => (
        <li key={v.version}>
          <div className="flex flex-wrap items-center gap-3 px-4 py-3">
            <button
              type="button"
              aria-expanded={open === v.version}
              onClick={() => setOpen(open === v.version ? undefined : v.version)}
              className="flex min-w-0 flex-1 items-center gap-3 rounded text-left outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              {open === v.version ? <CaretDownIcon className="size-4" /> : <CaretRightIcon className="size-4" />}
              <span className="font-medium tabular-nums">{t("Version {{version}}", { version: v.version })}</span>
              {v.version === set.version && <Pill tone="success">{t("Newest")}</Pill>}
              <span className="truncate text-sm text-muted-foreground">
                {v.user ? t("{{time}} by {{user}}", { time: formatDateTime(v.createdAt), user: v.user }) : formatDateTime(v.createdAt)}
              </span>
            </button>
            {editable && v.version !== set.version && <RestoreButton set={set} version={v.version} onRestore={onRestore} />}
          </div>
          {open === v.version && (
            <div className="border-t bg-muted/30 p-4">
              <VersionChanges set={set} version={v.version} previous={v.version > oldest ? v.version - 1 : undefined} />
            </div>
          )}
        </li>
      ))}
      {dirty && (
        <li className="px-4 py-3 text-sm text-muted-foreground">{t("Your unsaved changes become the next version when you save.")}</li>
      )}
    </ol>
  )
}

function RestoreButton({ set, version, onRestore }: { set: FileSet; version: number; onRestore: (files: SetFile[]) => void }) {
  const { data, isFetching, refetch } = useQuery({ ...versionQuery(set.id, version), enabled: false })
  return (
    <Button
      size="sm"
      variant="outline"
      disabled={isFetching}
      title={t("Load the files of this version into the editor, to save them as the newest version.")}
      onClick={async () => onRestore((data ?? (await refetch()).data)?.files ?? [])}
    >
      <ArrowCounterClockwiseIcon />
      {t("Restore")}
    </Button>
  )
}

/** What a version changed in the files of the version before it, if that is kept. */
function VersionChanges({ set, version, previous }: { set: FileSet; version: number; previous?: number }) {
  const current = useQuery(versionQuery(set.id, version))
  const before = useQuery({ ...versionQuery(set.id, previous ?? 0), enabled: previous !== undefined })
  if (current.isPending || (previous !== undefined && before.isPending)) return <Skeleton className="h-24 rounded-lg" />
  const error = current.error ?? before.error
  if (error) return <ErrorCallout error={error} />
  return <FilesDiff before={before.data?.files ?? []} after={current.data?.files ?? []} />
}

/** The files that differ between two states of a set, each with its changes; binary files only tell that they changed. */
function FilesDiff({ before, after }: { before: SetFile[]; after: SetFile[] }) {
  const paths = [...new Set([...before, ...after].map((f) => f.path))].sort()
  const changed = paths.filter((p) => {
    const a = before.find((f) => f.path === p)
    const b = after.find((f) => f.path === p)
    return a?.content !== b?.content || a?.data !== b?.data || a?.onlyIfMissing !== b?.onlyIfMissing
  })
  if (changed.length === 0) return <p className="text-sm text-muted-foreground">{t("The files didn't change.")}</p>
  return (
    <div className="grid gap-4">
      {changed.map((path) => {
        const a = before.find((f) => f.path === path)
        const b = after.find((f) => f.path === path)
        return (
          <div key={path} className="space-y-2">
            <p className="flex flex-wrap items-center gap-2 font-mono text-xs font-medium">
              {path}
              {!a && <Pill tone="success">{t("Added")}</Pill>}
              {!b && <Pill tone="destructive">{t("Removed")}</Pill>}
            </p>
            {a?.data !== undefined || b?.data !== undefined ? (
              <p className="text-sm text-muted-foreground">{t("A binary file, whose changes aren't shown.")}</p>
            ) : (
              <DiffView before={a?.content ?? ""} after={b?.content ?? ""} filename={path} label={t("Changes of {{path}}", { path })} />
            )}
          </div>
        )
      })}
    </div>
  )
}
