import { ArchiveIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { toast } from "sonner"
import { BackLink } from "@/components/back-link"
import { ErrorCallout } from "@/components/callout"
import { FormSection } from "@/components/form-section"
import { PageHeader } from "@/components/page-header"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { nodesQuery } from "@/features/nodes/api"
import type { TaskInput } from "@/features/schedules/api"
import { TaskForm } from "@/features/schedules/task-form"
import { emptyJob, type JobSettings, jobs } from "./api"
import { LocationField, SelectionField } from "./backup-fields"

const route = getRouteApi("/_app/backups/$jobId")

export function BackupJobPage() {
  const manage = useAccess().can("backupjobs.manage")
  const { jobId } = route.useParams()
  const { data: job, isPending, error } = useQuery(jobs.taskQuery(jobId))
  const save = jobs.useSaveTask(jobId)
  return (
    <>
      <BackLink to="/backups">{t("Backups")}</BackLink>
      {isPending ? (
        <Skeleton className="h-96 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : (
        <>
          <PageHeader icon={ArchiveIcon} tone="info" title={job.name} description={t("A job that backs up servers on a schedule.")} />
          {/* Without the permission to manage jobs, the job is only shown. */}
          <fieldset disabled={!manage} className="contents">
            <JobForm
              // Remounting on save resets the form to what was stored.
              key={JSON.stringify(job)}
              initial={{ name: job.name, enabled: job.enabled, schedule: job.schedule, targets: job.targets, settings: job.settings }}
              submitLabel={t("Save job")}
              save={save}
              onSaved={(j) => toast.success(t("Saved {{name}}", { name: j.name }))}
            />
          </fieldset>
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
      <BackLink to="/backups">{t("Backups")}</BackLink>
      <PageHeader
        icon={ArchiveIcon}
        tone="info"
        title={t("New backup job")}
        description={t("Back up servers or whole nodes on a schedule.")}
      />
      <JobForm
        initial={emptyJob}
        submitLabel={t("Create job")}
        save={save}
        onSaved={(j) => {
          toast.success(t("Created {{name}}", { name: j.name }))
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
      kind="job"
      submitLabel={submitLabel}
      pending={save.isPending}
      error={save.error}
      onSubmit={(input) => save.mutate(input, { onSuccess: onSaved })}
    >
      {(settings, set) => (
        <FormSection title={t("Backups")} description={t("What is backed up of each server, where it is kept and for how long.")}>
          <SelectionField value={settings.selection} onChange={(selection) => set({ selection })} />
          <LocationField locations={locations} value={settings.location} onChange={(location) => set({ location })} />
          <Field>
            <FieldLabel htmlFor="job-keep">{t("Backups to keep")}</FieldLabel>
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
              {t("Per server; older backups of this job are deleted. 0 keeps all of them. Backups made by hand are never deleted.")}
            </FieldDescription>
          </Field>
        </FormSection>
      )}
    </TaskForm>
  )
}
