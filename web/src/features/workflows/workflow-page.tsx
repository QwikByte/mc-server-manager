import { FlowArrowIcon, PlayIcon, PlusIcon, XIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useBlocker, useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { BackLink } from "@/components/back-link"
import { Callout, ErrorCallout } from "@/components/callout"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { FormSection } from "@/components/form-section"
import { PageHeader } from "@/components/page-header"
import { usePageName } from "@/components/page-title"
import { StatusBadge } from "@/components/status"
import { TimeZonePicker } from "@/components/time-zone-picker"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Field, FieldContent, FieldDescription, FieldError, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { useAccess } from "@/features/access/use-access"
import { msg } from "@/lib/i18n"
import { cn } from "@/lib/utils"
import { type Definition, type Draft, draftOf, emptyDraft, type Param, useSaveWorkflow, type Workflow, workflowQuery } from "./api"
import { EditorSheet } from "./editor-sheet"
import { examples } from "./examples"
import { FlowEditor } from "./flow"
import { RunNowDialog, Runs } from "./runs"
import { workflowStatus } from "./status"

const route = getRouteApi("/_app/workflows/$workflowId")
const newRoute = getRouteApi("/_app/workflows/new")

export function WorkflowPage() {
  const manage = useAccess().can("workflows.manage")
  const { workflowId } = route.useParams()
  const { data: workflow, isPending, error } = useQuery(workflowQuery(workflowId))
  usePageName(workflow?.name)
  return (
    <>
      <BackLink to="/workflows">{t("Workflows")}</BackLink>
      {isPending ? (
        <Skeleton className="h-[32rem] rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : (
        <>
          <PageHeader icon={FlowArrowIcon} tone="warning" title={workflow.name} badge={<StatusBadge status={workflowStatus(workflow)} />} />
          {!workflow.savedBy && (
            <Callout tone="warning" className="mb-6">
              {t("The user who saved it last was deleted or disabled, so its steps that need permissions fail. Save it again.")}
            </Callout>
          )}
          {/* Remounting on save resets the editor to what was stored. */}
          <Editor key={workflow.updatedAt} initial={draftOf(workflow)} workflow={workflow} manage={manage} />
          <Runs workflow={workflow} manage={manage} />
        </>
      )}
    </>
  )
}

export function NewWorkflowPage() {
  const { example } = newRoute.useSearch()
  usePageName(t("New workflow"))
  const [initial] = useState(() => (example !== undefined && examples[example] ? examples[example].draft() : emptyDraft))
  return (
    <>
      <BackLink to="/workflows">{t("Workflows")}</BackLink>
      <PageHeader icon={FlowArrowIcon} tone="warning" title={t("New workflow")} />
      <Editor initial={initial} manage />
    </>
  )
}

/** Edits a workflow: its name, the flow of its triggers and steps, and its settings, saved together. */
function Editor({ initial, workflow, manage }: { initial: Draft; workflow?: Workflow; manage: boolean }) {
  const [draft, setDraft] = useState(initial)
  const [selected, select] = useState<string>()
  const [asking, setAsking] = useState(false)
  const save = useSaveWorkflow(workflow?.id)
  const navigate = useNavigate()
  const dirty = JSON.stringify({ ...draft, name: draft.name.trim() }) !== JSON.stringify(initial)
  const blocker = useBlocker({ shouldBlockFn: () => dirty && !save.isPending, enableBeforeUnload: () => dirty, withResolver: true })
  const set = (change: Partial<Draft>) => setDraft({ ...draft, ...change })

  function submit(e: FormEvent) {
    e.preventDefault()
    save.mutate(
      { ...draft, name: draft.name.trim() },
      {
        onSuccess: (w) => {
          toast.success(workflow ? t("Saved {{name}}", { name: w.name }) : t("Created {{name}}", { name: w.name }))
          if (!workflow) void navigate({ to: "/workflows/$workflowId", params: { workflowId: w.id }, ignoreBlocker: true })
        },
      },
    )
  }

  return (
    <form onSubmit={submit} className="grid gap-6">
      <fieldset disabled={!manage} className="contents">
        <div className="surface grid gap-4 rounded-xl p-4 sm:grid-cols-[1fr_auto] sm:items-end sm:p-5">
          <Field>
            <FieldLabel htmlFor="workflow-name">{t("Name")}</FieldLabel>
            <Input id="workflow-name" required maxLength={64} value={draft.name} onChange={(e) => set({ name: e.target.value })} className="text-base font-semibold" />
          </Field>
          <Field orientation="horizontal" className="sm:pb-2">
            <Switch id="workflow-enabled" checked={draft.enabled} onCheckedChange={(enabled) => set({ enabled })} />
            <FieldLabel htmlFor="workflow-enabled">{t("Active")}</FieldLabel>
          </Field>
          <FieldDescription className="sm:col-span-2">
            {t("A paused workflow only runs when you start it, or when another workflow runs it.")}
          </FieldDescription>
        </div>

        {/* While the editor is open beside it, the flow moves aside so that it stays in view. */}
        <section
          aria-label={t("Flow")}
          className={cn(
            "rounded-2xl bg-[radial-gradient(circle,var(--color-border)_1px,transparent_1px)] [background-size:18px_18px] px-3 py-8 ring-1 ring-border transition-[padding] duration-300 sm:px-6",
            selected && "lg:pr-[24rem]",
          )}
        >
          <FlowEditor draft={draft} change={setDraft} selected={selected} select={select} readOnly={!manage} />
        </section>

        <div className="surface rounded-xl px-5 sm:px-8">
          <FormSection title={t("Description")}>
            <Field>
              <FieldLabel htmlFor="workflow-description" className="sr-only">
                {t("Description")}
              </FieldLabel>
              <Textarea id="workflow-description" maxLength={1000} rows={2} value={draft.description} placeholder={t("What it is for, for whoever reads it later.")} onChange={(e) => set({ description: e.target.value })} />
            </Field>
          </FormSection>
          <FormSection title={t("Inputs")}>
            <ParamsField value={draft.params} onChange={(params) => set({ params })} />
          </FormSection>
          <FormSection title={t("Runs")}>
            <OverlapField value={draft.overlap} onChange={(overlap) => set({ overlap })} />
            <Field>
              <FieldLabel htmlFor="workflow-zone">{t("Time zone")}</FieldLabel>
              <TimeZonePicker id="workflow-zone" value={draft.timeZone} onChange={(zone) => set({ timeZone: zone || "UTC" })} />
              <FieldDescription>{t("Of {{now}}, of delays until a time of day and of the days backups are kept by.", { now: "{{now}}" })}</FieldDescription>
            </Field>
          </FormSection>
        </div>
      </fieldset>

      {manage && (
        <div className="sticky bottom-4 z-10 flex flex-wrap-reverse items-center justify-end gap-x-4 gap-y-2 rounded-xl bg-card/90 px-4 py-3 shadow-lg ring-1 ring-border backdrop-blur-xl">
          {save.error && <FieldError className="mr-auto max-w-2xl">{save.error.message}</FieldError>}
          {!save.error && dirty && <span className="mr-auto text-sm text-muted-foreground">{t("Unsaved changes")}</span>}
          {workflow && !dirty && (
            <Button type="button" variant="outline" onClick={() => setAsking(true)}>
              <PlayIcon />
              {t("Run now")}
            </Button>
          )}
          {dirty && (
            <Button type="button" variant="ghost" onClick={() => setDraft(initial)}>
              {t("Discard")}
            </Button>
          )}
          <Button type="submit" disabled={(workflow && !dirty) || save.isPending}>
            {save.isPending ? t("Saving…") : workflow ? t("Save workflow") : t("Create workflow")}
          </Button>
        </div>
      )}
      <EditorSheet draft={draft} change={setDraft} selected={selected} select={select} workflowId={workflow?.id} hook={!!workflow?.hook} readOnly={!manage} />
      {workflow && <RunNowDialog workflow={workflow} open={asking} onOpenChange={setAsking} />}
      <ConfirmDialog
        open={blocker.status === "blocked"}
        onOpenChange={(open) => !open && blocker.reset?.()}
        title={t("Discard your changes?")}
        description={t("Your changes to the workflow haven't been saved.")}
        action={t("Discard changes")}
        destructive
        onConfirm={() => blocker.proceed?.()}
      />
    </form>
  )
}

const overlaps: { value: Definition["overlap"]; label: string; description: string }[] = [
  { value: "skip", label: msg("Skip"), description: msg("A trigger that fires while it runs starts nothing.") },
  { value: "queue", label: msg("Queue"), description: msg("It runs again once the run before ended, up to 20 times.") },
  { value: "parallel", label: msg("In parallel"), description: msg("Up to 10 runs at the same time.") },
]

function OverlapField({ value, onChange }: { value: Definition["overlap"]; onChange: (overlap: Definition["overlap"]) => void }) {
  return (
    <FieldSet>
      <FieldLegend variant="label">{t("While it runs")}</FieldLegend>
      <RadioGroup value={value} onValueChange={(v) => onChange(v as Definition["overlap"])} className="gap-2 sm:grid-cols-3">
        {overlaps.map((o) => (
          <FieldLabel key={o.value} htmlFor={`overlap-${o.value}`}>
            <Field orientation="horizontal" className="items-start">
              <FieldContent>
                <span className="font-medium">{t(o.label)}</span>
                <FieldDescription>{t(o.description)}</FieldDescription>
              </FieldContent>
              <RadioGroupItem id={`overlap-${o.value}`} value={o.value} />
            </Field>
          </FieldLabel>
        ))}
      </RadioGroup>
      <FieldDescription>{t("By hand and from other workflows, it always runs. Triggers start it at most 30 times at once, then every 10 seconds.")}</FieldDescription>
    </FieldSet>
  )
}

const paramTypes = () => [
  { value: "text", label: t("Text") },
  { value: "number", label: t("Number") },
  { value: "boolean", label: t("Yes or no") },
  { value: "data", label: t("Data (JSON)") },
]

/** The inputs a run gets, e.g. asked when it is started by hand. */
function ParamsField({ value, onChange }: { value: Param[]; onChange: (params: Param[]) => void }) {
  return (
    <>
      <FieldDescription>
        {t("Asked when you run it, and passed by workflows that run it; steps read them as {{example}}.", { example: "{{inputs.name}}" })}
      </FieldDescription>
      {value.length > 0 && (
        <ul className="grid gap-3">
          {value.map((p, i) => (
            // Inputs have no identity of their own; the fields are controlled.
            <li key={i} className={cn("grid gap-2 rounded-lg p-3 ring-1 ring-border sm:grid-cols-[1fr_9rem_1fr_auto]")}>
              <Input aria-label={t("Name")} placeholder={t("Name")} className="font-mono" maxLength={32} value={p.name} onChange={(e) => onChange(value.with(i, { ...p, name: e.target.value.replace(/[^A-Za-z0-9_]/g, "") }))} />
              <Select value={p.type} onValueChange={(type) => onChange(value.with(i, { ...p, type: type as Param["type"] }))}>
                <SelectTrigger aria-label={t("Type")} className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {paramTypes().map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Input aria-label={t("Default")} placeholder={t("Default")} value={p.default} onChange={(e) => onChange(value.with(i, { ...p, default: e.target.value }))} />
              <Button type="button" size="icon" variant="ghost" aria-label={t("Remove input {{name}}", { name: p.name })} onClick={() => onChange(value.toSpliced(i, 1))}>
                <XIcon />
              </Button>
              <Input aria-label={t("Description")} placeholder={t("Description")} className="sm:col-span-3" maxLength={200} value={p.description} onChange={(e) => onChange(value.with(i, { ...p, description: e.target.value }))} />
              <label className="flex items-center gap-2 text-sm">
                <Checkbox checked={p.required} onCheckedChange={(on) => onChange(value.with(i, { ...p, required: on === true }))} />
                {t("Required")}
              </label>
            </li>
          ))}
        </ul>
      )}
      <Button
        type="button"
        size="sm"
        variant="outline"
        className="w-fit"
        disabled={value.length >= 20}
        onClick={() => onChange([...value, { name: `input${value.length + 1}`, type: "text", default: "", required: false, description: "" }])}
      >
        <PlusIcon />
        {t("Input")}
      </Button>
    </>
  )
}
