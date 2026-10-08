import { TrashIcon, WarningIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { useState } from "react"
import { toast } from "sonner"
import { Callout } from "@/components/callout"
import { CopyField } from "@/components/copy-field"
import { IconTile } from "@/components/icon-tile"
import { Segmented } from "@/components/segmented"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Field, FieldContent, FieldDescription, FieldGroup, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Switch } from "@/components/ui/switch"
import { categories } from "@/features/logs/meta"
import { ScheduleField } from "@/features/schedules/schedule-field"
import { TaskTargetsField } from "@/features/schedules/targets"
import type { Draft, Measure, Step, Trigger } from "./api"
import { useDeleteHook, useNewHook } from "./api"
import { eventPresets, kinds, measures, serverEvents, stepName, triggers } from "./catalog"
import { StepFields } from "./step-fields"
import { DataContext } from "./data"
import { dataAt, findStep, remove, replace, walk } from "./tree"

/** Kinds whose templates are rendered for each server, with {{server}}. */
const perServer = new Set(["command", "message"])

/** Edits the step or trigger chosen in the flow, beside it; changes apply to the workflow at once. */
export function EditorSheet({
  draft,
  change,
  selected,
  select,
  workflowId,
  hook,
  readOnly,
}: {
  draft: Draft
  change: (draft: Draft) => void
  selected?: string
  select: (id?: string) => void
  workflowId?: string
  /** Whether the saved workflow has a URL. */
  hook: boolean
  readOnly: boolean
}) {
  const triggerIndex = selected?.startsWith("trigger:") ? Number(selected.slice(8)) : -1
  const trigger = draft.triggers[triggerIndex]
  const step = triggerIndex < 0 && selected ? findStep(draft.steps, selected) : undefined
  return (
    // Beside the flow rather than over it, so that clicking another card edits that one.
    <Sheet open={!!(trigger || step)} onOpenChange={(open) => !open && select(undefined)} modal={false}>
      <SheetContent className="w-full gap-0 sm:max-w-xl" aria-describedby={undefined} onInteractOutside={(e) => e.preventDefault()}>
        <fieldset disabled={readOnly} className="contents">
          {step && <StepEditor draft={draft} change={change} step={step} select={select} workflowId={workflowId} />}
          {trigger && (
            <TriggerEditor
              trigger={trigger}
              onChange={(tr) => change({ ...draft, triggers: draft.triggers.with(triggerIndex, tr) })}
              onDelete={() => {
                change({ ...draft, triggers: draft.triggers.toSpliced(triggerIndex, 1) })
                select(undefined)
              }}
              workflowId={workflowId}
              hook={hook}
            />
          )}
        </fieldset>
      </SheetContent>
    </Sheet>
  )
}

function StepEditor({ draft, change, step, select, workflowId }: {
  draft: Draft
  change: (draft: Draft) => void
  step: Step
  select: (id?: string) => void
  workflowId?: string
}) {
  const info = kinds[step.kind]
  const set = (s: Step) => change({ ...draft, steps: replace(draft.steps, s) })
  const finds = walk(draft.steps).flatMap(({ step: s }) => (s.kind === "servers" && s.id !== step.id ? [s.id] : []))
  return (
    <>
      <SheetHeader className="border-b">
        <div className="flex items-center gap-3 pr-8">
          {info && <IconTile icon={info.icon} tone={info.tone} />}
          <div className="min-w-0">
            <SheetTitle className="truncate">{stepName(step)}</SheetTitle>
            <SheetDescription className="truncate">{info && t(info.description)}</SheetDescription>
          </div>
        </div>
      </SheetHeader>
      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-5">
        <DataContext.Provider value={dataAt(draft, step.id, perServer.has(step.kind))}>
          <FieldGroup>
            <div className="grid gap-4 sm:grid-cols-[1fr_auto]">
              <Field>
                <FieldLabel htmlFor="step-name">{t("Name")}</FieldLabel>
                <Input id="step-name" maxLength={64} placeholder={info ? t(info.label) : ""} value={step.name ?? ""} onChange={(e) => set({ ...step, name: e.target.value })} />
              </Field>
              <Field>
                <FieldLabel htmlFor="step-id">{t("Its data")}</FieldLabel>
                <code id="step-id" className="flex h-9 items-center rounded-lg bg-muted px-2.5 font-mono text-xs text-muted-foreground">
                  {`steps.${step.id}`}
                </code>
              </Field>
            </div>
            <StepFields step={step} onChange={set} workflowId={workflowId} finds={finds} />
            <FieldSet className="border-t pt-5">
              <FieldLegend variant="label">{t("When it fails")}</FieldLegend>
              <Field orientation="horizontal">
                <Switch id="step-continue" checked={!!step.continue} onCheckedChange={(on) => set({ ...step, continue: on })} />
                <FieldContent>
                  <FieldLabel htmlFor="step-continue">{t("Go on with the next step")}</FieldLabel>
                  <FieldDescription>{t("Its outcome and error tell later steps what happened, e.g. for a condition.")}</FieldDescription>
                </FieldContent>
              </Field>
              <Field orientation="horizontal">
                <Switch id="step-disabled" checked={!!step.disabled} onCheckedChange={(on) => set({ ...step, disabled: on })} />
                <FieldContent>
                  <FieldLabel htmlFor="step-disabled">{t("Turned off")}</FieldLabel>
                  <FieldDescription>{t("Runs skip it, with the steps within it.")}</FieldDescription>
                </FieldContent>
              </Field>
            </FieldSet>
          </FieldGroup>
        </DataContext.Provider>
      </div>
      <SheetFooter className="flex-row justify-between border-t">
        <Button
          type="button"
          variant="ghost"
          className="text-destructive hover:bg-destructive/10 hover:text-destructive"
          onClick={() => {
            change({ ...draft, steps: remove(draft.steps, step.id) })
            select(undefined)
          }}
        >
          <TrashIcon />
          {t("Delete step")}
        </Button>
        <Button type="button" onClick={() => select(undefined)}>
          {t("Done")}
        </Button>
      </SheetFooter>
    </>
  )
}

function TriggerEditor({ trigger, onChange, onDelete, workflowId, hook }: {
  trigger: Trigger
  onChange: (trigger: Trigger) => void
  onDelete: () => void
  workflowId?: string
  hook: boolean
}) {
  const info = triggers[trigger.kind]
  const set = (change: Partial<Trigger>) => onChange({ ...trigger, ...change })
  return (
    <>
      <SheetHeader className="border-b">
        <div className="flex items-center gap-3 pr-8">
          <IconTile icon={info.icon} tone="warning" />
          <div className="min-w-0">
            <SheetTitle>{t(info.label)}</SheetTitle>
            <SheetDescription>{t(info.description)}</SheetDescription>
          </div>
        </div>
      </SheetHeader>
      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-5">
        <FieldGroup>
          {trigger.kind === "schedule" && trigger.schedule && <ScheduleField value={trigger.schedule} onChange={(schedule) => set({ schedule })} />}
          {trigger.kind === "interval" && <IntervalField value={trigger.every ?? 15} onChange={(every) => set({ every })} />}
          {trigger.kind === "event" && <EventFields trigger={trigger} set={set} />}
          {trigger.kind === "server" && <ServerFields trigger={trigger} set={set} />}
          {trigger.kind === "metric" && <MetricFields trigger={trigger} set={set} />}
          {trigger.kind === "webhook" && <HookField workflowId={workflowId} hook={hook} />}
          {(trigger.kind === "event" || trigger.kind === "server" || trigger.kind === "metric") && (
            <FieldSet className="border-t pt-5">
              <FieldLegend variant="label">{t("Only for these servers")}</FieldLegend>
              <FieldDescription>{t("None chosen watches all servers.")}</FieldDescription>
              <TaskTargetsField value={trigger.targets ?? []} onChange={(targets) => set({ targets })} />
            </FieldSet>
          )}
        </FieldGroup>
      </div>
      <SheetFooter className="flex-row justify-between border-t">
        <Button type="button" variant="ghost" className="text-destructive hover:bg-destructive/10 hover:text-destructive" onClick={onDelete}>
          <TrashIcon />
          {t("Delete trigger")}
        </Button>
      </SheetFooter>
    </>
  )
}

function IntervalField({ value, onChange }: { value: number; onChange: (minutes: number) => void }) {
  const hours = value % 60 === 0
  return (
    <Field>
      <FieldLabel htmlFor="trigger-every">{t("Every")}</FieldLabel>
      <div className="flex items-center gap-2">
        <Input
          id="trigger-every"
          type="number"
          min={1}
          className="w-28 font-mono"
          value={hours ? value / 60 : value}
          onChange={(e) => onChange(Math.max(1, e.target.valueAsNumber || 1) * (hours ? 60 : 1))}
        />
        <Segmented
          label={t("Unit")}
          value={hours ? "hours" : "minutes"}
          onChange={(unit) => onChange(unit === "hours" ? Math.max(1, Math.round(value / 60)) * 60 : value === 60 ? 59 : value)}
          options={[
            { value: "minutes", label: t("minutes") },
            { value: "hours", label: t("hours") },
          ]}
        />
      </div>
      <FieldDescription>{t("Runs at whole multiples, e.g. every 15 minutes at :00, :15, :30 and :45.")}</FieldDescription>
    </Field>
  )
}

function EventFields({ trigger, set }: { trigger: Trigger; set: (change: Partial<Trigger>) => void }) {
  const chosen = trigger.categories ?? []
  return (
    <>
      <FieldSet>
        <FieldLegend variant="label">{t("Ready to use")}</FieldLegend>
        <div className="flex flex-wrap gap-2">
          {eventPresets.map((p) => (
            <Button key={p.label} type="button" size="sm" variant="outline" onClick={() => set({ level: p.level, categories: [...p.categories], contains: p.contains })}>
              {t(p.label)}
            </Button>
          ))}
        </div>
      </FieldSet>
      <Field>
        <FieldLabel htmlFor="trigger-level">{t("At least the level")}</FieldLabel>
        <Select value={trigger.level ?? "warn"} onValueChange={(level) => set({ level: level as Trigger["level"] })}>
          <SelectTrigger id="trigger-level" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="info">{t("Info")}</SelectItem>
            <SelectItem value="warn">{t("Warning")}</SelectItem>
            <SelectItem value="error">{t("Error")}</SelectItem>
          </SelectContent>
        </Select>
      </Field>
      <Field>
        <FieldLabel htmlFor="trigger-contains">{t("Message contains")}</FieldLabel>
        <Input id="trigger-contains" maxLength={200} value={trigger.contains ?? ""} onChange={(e) => set({ contains: e.target.value })} />
        <FieldDescription>{t("Regardless of case; empty for any message. The log names entries in English.")}</FieldDescription>
      </Field>
      <FieldSet>
        <FieldLegend variant="label">{t("Categories")}</FieldLegend>
        <FieldDescription>{t("None chosen means all.")}</FieldDescription>
        <div className="grid grid-cols-2 gap-2">
          {Object.entries(categories).map(([id, label]) => (
            <label key={id} className="flex items-center gap-2 text-sm">
              <Checkbox checked={chosen.includes(id)} onCheckedChange={(on) => set({ categories: on ? [...chosen, id] : chosen.filter((c) => c !== id) })} />
              {t(label)}
            </label>
          ))}
        </div>
      </FieldSet>
    </>
  )
}

function ServerFields({ trigger, set }: { trigger: Trigger; set: (change: Partial<Trigger>) => void }) {
  const players = trigger.on === "joined" || trigger.on === "left"
  // Kept as typed, so that a comma can be entered before the next name.
  const [names, setNames] = useState((trigger.players ?? []).join(", "))
  return (
    <>
      <Field>
        <FieldLabel htmlFor="trigger-on">{t("When")}</FieldLabel>
        <Select value={trigger.on ?? "joined"} onValueChange={(on) => set({ on: on as Trigger["on"], players: [] })}>
          <SelectTrigger id="trigger-on" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(serverEvents).map(([on, label]) => (
              <SelectItem key={on} value={on}>
                {t(label)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <FieldDescription>{t("The master notices it at the next measurement, once a minute.")}</FieldDescription>
      </Field>
      {players && (
        <Field>
          <FieldLabel htmlFor="trigger-players">{t("Only these players")}</FieldLabel>
          <Input
            id="trigger-players"
            value={names}
            onChange={(e) => {
              setNames(e.target.value)
              set({ players: e.target.value.split(/[\s,]+/).filter(Boolean) })
            }}
          />
          <FieldDescription>{t("Names separated by commas; empty for everyone.")}</FieldDescription>
        </Field>
      )}
    </>
  )
}

function MetricFields({ trigger, set }: { trigger: Trigger; set: (change: Partial<Trigger>) => void }) {
  const measure = measures[trigger.measure ?? "players"]
  return (
    <>
      <Field>
        <FieldLabel htmlFor="trigger-measure">{t("Measure")}</FieldLabel>
        <Select value={trigger.measure ?? "players"} onValueChange={(m) => set({ measure: m as Measure })}>
          <SelectTrigger id="trigger-measure" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(measures).map(([m, info]) => (
              <SelectItem key={m} value={m}>
                {t(info.label)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <div className="flex flex-wrap items-end gap-3">
        <Segmented
          label={t("Direction")}
          value={trigger.below ? "below" : "above"}
          onChange={(d) => set({ below: d === "below" })}
          options={[
            { value: "above", label: t("above") },
            { value: "below", label: t("below") },
          ]}
        />
        <Input aria-label={t("Value")} type="number" step="any" className="w-28 font-mono" value={trigger.value ?? 0} onChange={(e) => set({ value: e.target.valueAsNumber || 0 })} />
        <span className="pb-2 text-sm text-muted-foreground">{measure.unit}</span>
      </div>
      <Field>
        <FieldLabel htmlFor="trigger-minutes">{t("For at least")}</FieldLabel>
        <div className="flex items-center gap-2">
          <Input id="trigger-minutes" type="number" min={0} max={60} className="w-24 font-mono" value={trigger.minutes ?? 0} onChange={(e) => set({ minutes: e.target.valueAsNumber || 0 })} />
          <span className="text-sm text-muted-foreground">{t("minutes")}</span>
        </div>
        <FieldDescription>
          {t("It fires once per server, and again only after the measure was back. CPU is in percent of the server's limit, or of all cores of its node.")}
        </FieldDescription>
      </Field>
    </>
  )
}

/** The URL that starts a workflow: created after it is saved, and shown only then. */
function HookField({ workflowId, hook }: { workflowId?: string; hook: boolean }) {
  const create = useNewHook(workflowId ?? "")
  const drop = useDeleteHook(workflowId ?? "")
  const [url, setUrl] = useState<string>()
  if (!workflowId) return <Callout>{t("Save the workflow first; then it gets its URL here.")}</Callout>
  return (
    <FieldSet>
      <FieldLegend variant="label">{t("URL")}</FieldLegend>
      {url ? (
        <>
          <CopyField label={t("URL")} value={url} />
          <Callout tone="warning" icon={WarningIcon}>
            {t("Copy it now: it is shown only once. Anyone with it can start the workflow, so keep it secret.")}
          </Callout>
          <CopyField label={t("Example")} prefix="$" value={`curl -X POST -H 'Content-Type: application/json' -d '{"text":"hello"}' ${url}`} />
        </>
      ) : (
        <FieldDescription>
          {hook
            ? t("It has a URL. A new one replaces it, and the old one stops working.")
            : t("It has no URL yet. Calls of it start the workflow with their JSON as {{body}}.", { body: "{{trigger.body}}" })}
        </FieldDescription>
      )}
      <div className="flex flex-wrap gap-2">
        <Button
          type="button"
          size="sm"
          disabled={create.isPending}
          onClick={() =>
            create.mutate(undefined, {
              onSuccess: (res) => setUrl(new URL(res.path, location.origin).href),
              onError: (e) => toast.error(e.message),
            })
          }
        >
          {hook || url ? t("New URL") : t("Create URL")}
        </Button>
        {(hook || url) && (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={drop.isPending}
            onClick={() =>
              drop.mutate(undefined, {
                onSuccess: () => {
                  setUrl(undefined)
                  toast.success(t("Removed the URL"))
                },
                onError: (e) => toast.error(e.message),
              })
            }
          >
            {t("Remove URL")}
          </Button>
        )}
      </div>
      <FieldDescription>{t("Calls send POST with up to 64 KB, as JSON or text. The workflow has to be active.")}</FieldDescription>
    </FieldSet>
  )
}
