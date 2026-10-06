import { FilesIcon, PlusIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { EmptyState } from "@/components/empty-state"
import { PageHeader } from "@/components/page-header"
import { Pill, StatusDot } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { networksQuery } from "@/features/networks/api"
import { fileSetsQuery, type State, type Summary, statusesQuery } from "./api"
import { states, targetKey, targetLabel } from "./labels"
import { NewFileSetDialog } from "./new-fileset-dialog"

function NewFileSet() {
  return (
    <NewFileSetDialog
      trigger={
        <Button>
          <PlusIcon />
          {t("New file set")}
        </Button>
      }
    />
  )
}

export function FileSetsPage() {
  const manage = useAccess().can("filesets.manage")
  const { data: sets, isPending, error } = useQuery(fileSetsQuery)
  return (
    <>
      <PageHeader
        icon={FilesIcon}
        tone="info"
        title={t("File sets")}
        description={t("Configuration files that many servers share, e.g. of plugins, kept in one place and applied to the servers of tags and networks.")}
        actions={manage && <NewFileSet />}
      />
      {isPending ? (
        <div className="grid gap-4 md:grid-cols-2">
          {[0, 1].map((i) => (
            <Skeleton key={i} className="h-44 rounded-xl" />
          ))}
        </div>
      ) : error ? (
        <ErrorCallout error={error} />
      ) : sets.length === 0 ? (
        <EmptyState
          icon={FilesIcon}
          tone="info"
          title={t("No file sets yet")}
          description={manage && t("Create a set, add the files it puts on servers, and choose the tags and networks it is for.")}
        >
          {manage && <NewFileSet />}
        </EmptyState>
      ) : (
        <ul className="grid gap-4 md:grid-cols-2">
          {sets.map((set) => (
            <FileSetCard key={set.id} set={set} />
          ))}
        </ul>
      )}
    </>
  )
}

function FileSetCard({ set }: { set: Summary }) {
  const { data: networks } = useQuery(networksQuery)
  const { data: statuses } = useQuery(statusesQuery)
  const counts = new Map<State, number>()
  for (const s of statuses?.[set.id] ?? []) counts.set(s.state, (counts.get(s.state) ?? 0) + 1)
  return (
    <li className="surface flex flex-col gap-4 rounded-xl p-5">
      <div className="min-w-0 space-y-1">
        <Link to="/filesets/$fileSetId" params={{ fileSetId: set.id }} className="block truncate font-semibold hover:underline">
          {set.name}
        </Link>
        <p className="text-xs text-muted-foreground">
          {t("Version {{version}} · {{count}} files", { version: set.version, count: set.paths.length, defaultValue_one: "Version {{version}} · {{count}} file" })}
        </p>
      </div>
      {set.description && <p className="line-clamp-2 text-sm text-muted-foreground">{set.description}</p>}
      <div className="flex flex-wrap gap-1.5">
        {set.targets.length === 0 ? (
          <span className="text-sm text-muted-foreground">{t("No targets yet")}</span>
        ) : (
          set.targets.map((target) => {
            const { icon, label } = targetLabel(target, networks)
            return (
              <Chip key={targetKey(target)} icon={icon} className="font-normal">
                {label}
              </Chip>
            )
          })
        )}
      </div>
      {counts.size > 0 && (
        <div className="mt-auto flex flex-wrap gap-1.5 border-t pt-4">
          {(Object.keys(states) as State[])
            .filter((s) => counts.has(s))
            .map((s) => (
              <Pill key={s} tone={states[s].tone}>
                <StatusDot status={states[s]} />
                <span className="tabular-nums">{counts.get(s)}</span> {t(states[s].label)}
              </Pill>
            ))}
        </div>
      )}
    </li>
  )
}
