import { ArrowsClockwiseIcon, UsersThreeIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { StatusBadge } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { type FileSet, type ServerStatus, statusQuery } from "./api"
import { states } from "./labels"

/** Which servers have the newest files of a set, which have older or changed ones, and which still have it after they left. */
export function ServersTab({ set }: { set: FileSet }) {
  const { data: list, isPending, isFetching, error, refetch } = useQuery(statusQuery(set.id))
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">{t("The servers the set is for, and those that still have its files.")}</p>
        <Button variant="outline" size="sm" disabled={isFetching} onClick={() => void refetch()}>
          <ArrowsClockwiseIcon className={isFetching ? "animate-spin motion-reduce:animate-none" : undefined} />
          {t("Check now")}
        </Button>
      </div>
      {isPending ? (
        <Skeleton className="h-40 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : list.length === 0 ? (
        <EmptyState icon={UsersThreeIcon} tone="info" title={t("The set is for no server yet")} description={t("Add a tag or a network as a target.")} />
      ) : (
        <div className="surface overflow-hidden rounded-xl">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("Server")}</TableHead>
                <TableHead>{t("State")}</TableHead>
                <TableHead>{t("Version")}</TableHead>
                <TableHead>{t("Details")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {list.map((s) => (
                <StatusRow key={`${s.nodeId}/${s.serverId}`} status={s} />
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}

function StatusRow({ status: s }: { status: ServerStatus }) {
  const state = states[s.state]
  return (
    <TableRow>
      <TableCell>
        <Link
          to="/nodes/$nodeId/servers/$serverId/files"
          params={{ nodeId: s.nodeId, serverId: s.serverId }}
          className="font-medium hover:underline"
        >
          {s.name || s.serverId}
        </Link>
        <p className="text-xs text-muted-foreground">{s.nodeName}</p>
      </TableCell>
      <TableCell>
        <span title={t(state.description)}>
          <StatusBadge status={state} />
        </span>
      </TableCell>
      <TableCell className="text-sm tabular-nums">{s.version ?? "–"}</TableCell>
      <TableCell className="max-w-md text-sm">
        {s.problem && <p className="text-destructive">{s.problem}</p>}
        {s.changed && s.changed.length > 0 && (
          <p className="truncate text-muted-foreground" title={s.changed.join("\n")}>
            {t("Changed: {{files}}", { files: s.changed.join(", ") })}
          </p>
        )}
      </TableCell>
    </TableRow>
  )
}
