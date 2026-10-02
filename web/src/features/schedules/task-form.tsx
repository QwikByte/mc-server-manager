import { useBlocker } from "@tanstack/react-router"
import { type FormEvent, type ReactNode, useState } from "react"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { FormSection } from "@/components/form-section"
import { Button } from "@/components/ui/button"
import { Field, FieldContent, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import type { TaskInput } from "./api"
import { ScheduleField } from "./schedule-field"
import { TargetsField } from "./targets-field"

/**
 * Edits a task: its name, schedule and servers, and the settings of its kind, which
 * children renders as further sections.
 */
export function TaskForm<S>({
  initial,
  noun,
  submitLabel,
  pending,
  error,
  onSubmit,
  children,
}: {
  initial: TaskInput<S>
  /** What the task is called, e.g. "job" or "policy". */
  noun: string
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
      <FormSection title="General" description={`What the ${noun} is called, and whether it runs on its schedule.`}>
        <Field>
          <FieldLabel htmlFor="task-name">Name</FieldLabel>
          <Input id="task-name" required maxLength={64} value={form.name} onChange={(e) => set({ name: e.target.value })} />
        </Field>
        <Field orientation="horizontal">
          <Switch id="task-enabled" checked={form.enabled} onCheckedChange={(enabled) => set({ enabled })} />
          <FieldContent>
            <FieldLabel htmlFor="task-enabled">Active</FieldLabel>
            <FieldDescription>A paused {noun} only runs when you start it.</FieldDescription>
          </FieldContent>
        </Field>
      </FormSection>

      {children(form.settings, (change) => set({ settings: { ...form.settings, ...change } }))}

      <FormSection title="Schedule" description="When it runs. Runs missed while the master was down are skipped.">
        <ScheduleField value={form.schedule} onChange={(schedule) => set({ schedule })} />
      </FormSection>

      <FormSection title="Servers" description="Where it runs. A whole node includes the servers created later.">
        <TargetsField value={form.targets} onChange={(targets) => set({ targets })} />
      </FormSection>

      <div className="-mx-5 flex flex-wrap-reverse items-center justify-end gap-x-6 gap-y-3 rounded-b-2xl bg-muted/50 px-5 py-4 sm:-mx-8 sm:px-8">
        {error && <FieldError className="mr-auto">{error.message}</FieldError>}
        <Button type="submit" disabled={!dirty || pending || form.targets.length === 0}>
          {pending ? "Saving…" : submitLabel}
        </Button>
      </div>
      <ConfirmDialog
        open={blocker.status === "blocked"}
        onOpenChange={(open) => !open && blocker.reset?.()}
        title="Discard your changes?"
        description={`Your changes to the ${noun} haven't been saved.`}
        action="Discard changes"
        destructive
        onConfirm={() => blocker.proceed?.()}
      />
    </form>
  )
}
