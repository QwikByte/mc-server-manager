import { useBlocker } from "@tanstack/react-router"
import { t } from "i18next"
import { type FormEvent, type ReactNode, useState } from "react"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { FormSection } from "@/components/form-section"
import { Button } from "@/components/ui/button"
import { Field, FieldContent, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { msg } from "@/lib/i18n"
import type { TaskInput } from "./api"
import { ScheduleField } from "./schedule-field"
import { TargetsField } from "@/features/servers/targets-field"

const texts = {
  job: {
    general: msg("What the job is called, and whether it runs on its schedule."),
    paused: msg("A paused job only runs when you start it."),
    unsaved: msg("Your changes to the job haven't been saved."),
  },
  policy: {
    general: msg("What the policy is called, and whether it runs on its schedule."),
    paused: msg("A paused policy only runs when you start it."),
    unsaved: msg("Your changes to the policy haven't been saved."),
  },
}

/**
 * Edits a task: its name, schedule and servers, and the settings of its kind, which
 * children renders as further sections.
 */
export function TaskForm<S>({
  initial,
  kind,
  submitLabel,
  pending,
  error,
  onSubmit,
  children,
}: {
  initial: TaskInput<S>
  kind: keyof typeof texts
  submitLabel: string
  pending: boolean
  error: Error | null
  onSubmit: (input: TaskInput<S>) => void
  children: (settings: S, set: (change: Partial<S>) => void) => ReactNode
}) {
  const [form, setForm] = useState(initial)
  const dirty = JSON.stringify({ ...form, name: form.name.trim() }) !== JSON.stringify(initial)
  const blocker = useBlocker({ shouldBlockFn: () => dirty && !pending, enableBeforeUnload: () => dirty, withResolver: true })
  const set = (change: Partial<TaskInput<S>>) => setForm({ ...form, ...change })

  function submit(event: FormEvent) {
    event.preventDefault()
    onSubmit({ ...form, name: form.name.trim() })
  }

  return (
    <form onSubmit={submit} className="surface rounded-2xl px-5 sm:px-8">
      <FormSection title={t("General")} description={t(texts[kind].general)}>
        <Field>
          <FieldLabel htmlFor="task-name">{t("Name")}</FieldLabel>
          <Input id="task-name" required maxLength={64} value={form.name} onChange={(e) => set({ name: e.target.value })} />
        </Field>
        <Field orientation="horizontal">
          <Switch id="task-enabled" checked={form.enabled} onCheckedChange={(enabled) => set({ enabled })} />
          <FieldContent>
            <FieldLabel htmlFor="task-enabled">{t("Active")}</FieldLabel>
            <FieldDescription>{t(texts[kind].paused)}</FieldDescription>
          </FieldContent>
        </Field>
      </FormSection>

      {children(form.settings, (change) => set({ settings: { ...form.settings, ...change } }))}

      <FormSection title={t("Schedule")} description={t("When it runs. Runs missed while the master was down are skipped.")}>
        <ScheduleField value={form.schedule} onChange={(schedule) => set({ schedule })} />
      </FormSection>

      <FormSection title={t("Servers")} description={t("Where it runs. A whole node includes the servers created later.")}>
        <TargetsField value={form.targets} onChange={(targets) => set({ targets })} />
      </FormSection>

      <div className="-mx-5 flex flex-wrap-reverse items-center justify-end gap-x-6 gap-y-3 rounded-b-2xl bg-muted/50 px-5 py-4 sm:-mx-8 sm:px-8">
        {error && <FieldError className="mr-auto">{error.message}</FieldError>}
        <Button type="submit" disabled={!dirty || pending || form.targets.length === 0}>
          {pending ? t("Saving…") : submitLabel}
        </Button>
      </div>
      <ConfirmDialog
        open={blocker.status === "blocked"}
        onOpenChange={(open) => !open && blocker.reset?.()}
        title={t("Discard your changes?")}
        description={t(texts[kind].unsaved)}
        action={t("Discard changes")}
        destructive
        onConfirm={() => blocker.proceed?.()}
      />
    </form>
  )
}
