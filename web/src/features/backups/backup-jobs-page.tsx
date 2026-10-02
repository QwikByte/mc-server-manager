import { ArchiveIcon, PencilSimpleIcon, PlusIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { TaskCard } from "@/features/schedules/task-card"
import { describeSelection, jobs } from "./api"

const newJob = (
  <Button asChild>
    <Link to="/backups/new">
      <PlusIcon />
      New backup job
    </Link>
  </Button>
)

export function BackupJobsPage() {
  const { data: list, isPending, error } = useQuery(jobs.tasksQuery)
  return (
    <>
      <PageHeader
        icon={ArchiveIcon}
        tone="info"
        title="Backups"
        description="Jobs back up servers on a schedule and keep their newest backups. The backups of a server are in its Backups tab."
        actions={newJob}
      />
      {isPending ? (
        <div className="grid gap-4 md:grid-cols-2">
          {[0, 1].map((i) => (
            <Skeleton key={i} className="h-60 rounded-xl" />
          ))}
        </div>
      ) : error ? (
        <ErrorCallout error={error} />
      ) : list.length === 0 ? (
        <EmptyState
          icon={ArchiveIcon}
          tone="info"
          title="No backup jobs yet"
          description="Create a job to back up servers or whole nodes every night, for example."
        >
          {newJob}
        </EmptyState>
      ) : (
        <ul className="grid gap-4 md:grid-cols-2">
          {list.map((job) => (
            <TaskCard
              key={job.id}
              task={job}
              taskApi={jobs}
              icon={ArchiveIcon}
              tone="info"
              title={
                <Link to="/backups/$jobId" params={{ jobId: job.id }} className="block truncate font-semibold hover:underline">
                  {job.name}
                </Link>
              }
              summary={`${describeSelection(job.settings.selection)} · ${job.settings.keep ? `keeps ${job.settings.keep}` : "keeps all"}`}
              edit={
                <Button asChild size="sm" variant="outline">
                  <Link to="/backups/$jobId" params={{ jobId: job.id }}>
                    <PencilSimpleIcon />
                    Edit
                  </Link>
                </Button>
              }
            />
          ))}
        </ul>
      )}
    </>
  )
}
