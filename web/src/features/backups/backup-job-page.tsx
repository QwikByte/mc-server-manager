import { ArchiveIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useNavigate } from "@tanstack/react-router"
import { toast } from "sonner"
import { BackLink } from "@/components/back-link"
import { ErrorCallout } from "@/components/callout"
import { FormSection } from "@/components/form-section"
import { PageHeader } from "@/components/page-header"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { nodesQuery } from "@/features/nodes/api"
import type { TaskInput } from "@/features/schedules/api"
import { TaskForm } from "@/features/schedules/task-form"
import { emptyJob, type JobSettings, jobs } from "./api"
import { LocationField, SelectionField } from "./backup-fields"

const route = getRouteApi("/_app/backups/$jobId")

export function BackupJobPage() {
  const { jobId } = route.useParams()
  const { data: job, isPending, error } = useQuery(jobs.taskQuery(jobId))
  const save = jobs.useSaveTask(jobId)
  return (
    <>
      <BackLink to="/backups">Backups</BackLink>
      {isPending ? (
        <Skeleton className="h-96 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : (
        <>
          <PageHeader icon={ArchiveIcon} tone="info" title={job.name} description="A job that backs up servers on a schedule." />
          <JobForm
            // Remounting on save resets the form to what was stored.
            key={JSON.stringify(job)}
            initial={{ name: job.name, enabled: job.enabled, schedule: job.schedule, targets: job.targets, settings: job.settings }}
            submitLabel="Save job"
            save={save}
            onSaved={(j) => toast.success(`Saved ${j.name}`)}
          />
        </>
      )}
    </>
  )
}

export function NewBackupJobPage() {
  const save = jobs.useSaveTask()
  const navigate = useNavigate()
  return (
    <>
      <BackLink to="/backups">Backups</BackLink>
      <PageHeader icon={ArchiveIcon} tone="info" title="New backup job" description="Back up servers or whole nodes on a schedule." />
      <JobForm
        initial={emptyJob}
        submitLabel="Create job"
        save={save}
        onSaved={(j) => {
          toast.success(`Created ${j.name}`)
          void navigate({ to: "/backups", ignoreBlocker: true })
        }}
      />
    </>
  )
}

function JobForm({
  initial,
  submitLabel,
  save,
  onSaved,
}: {
  initial: TaskInput<JobSettings>
  submitLabel: string
  save: ReturnType<typeof jobs.useSaveTask>
  onSaved: (job: { name: string }) => void
}) {
  const { data: nodes = [] } = useQuery(nodesQuery)
  const locations = nodes.flatMap((n) => n.info?.storage.map((l) => l.name) ?? [])
  return (
    <TaskForm
      initial={initial}
      noun="job"
      submitLabel={submitLabel}
      pending={save.isPending}
      error={save.error}
      onSubmit={(input) => save.mutate(input, { onSuccess: onSaved })}
    >
      {(settings, set) => (
        <FormSection title="Backups" description="What is backed up of each server, where it is kept and for how long.">
          <SelectionField value={settings.selection} onChange={(selection) => set({ selection })} />
          <LocationField locations={locations} value={settings.location} onChange={(location) => set({ location })} />
          <Field>
            <FieldLabel htmlFor="job-keep">Backups to keep</FieldLabel>
            <Input
              id="job-keep"
              type="number"
              min={0}
              max={1000}
              className="w-full font-mono sm:w-32"
              value={settings.keep}
              onChange={(e) => set({ keep: e.target.valueAsNumber || 0 })}
            />
            <FieldDescription>
              Per server; older backups of this job are deleted. 0 keeps all of them. Backups made by hand are never deleted.
            </FieldDescription>
          </Field>
        </FormSection>
      )}
    </TaskForm>
  )
}
