import { CalendarCheckIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useNavigate } from "@tanstack/react-router"
import { useState } from "react"
import { toast } from "sonner"
import { BackLink } from "@/components/back-link"
import { ErrorCallout } from "@/components/callout"
import { FormSection } from "@/components/form-section"
import { PageHeader } from "@/components/page-header"
import { Field, FieldContent, FieldDescription, FieldLabel, FieldTitle } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Skeleton } from "@/components/ui/skeleton"
import type { TaskInput } from "@/features/schedules/api"
import { TaskForm } from "@/features/schedules/task-form"
import { actions, emptyPolicy, type PolicyAction, type PolicySettings, policies, warns } from "./api"

const route = getRouteApi("/_app/policies/$policyId")

export function PolicyPage() {
  const { policyId } = route.useParams()
  const { data: policy, isPending, error } = useQuery(policies.taskQuery(policyId))
  const save = policies.useSaveTask(policyId)
  return (
    <>
      <BackLink to="/policies">Policies</BackLink>
      {isPending ? (
        <Skeleton className="h-96 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : (
        <>
          <PageHeader icon={CalendarCheckIcon} tone="warning" title={policy.name} description={actions[policy.settings.action].description} />
          <PolicyForm
            // Remounting on save resets the form to what was stored.
            key={JSON.stringify(policy)}
            initial={{ name: policy.name, enabled: policy.enabled, schedule: policy.schedule, targets: policy.targets, settings: policy.settings }}
            submitLabel="Save policy"
            save={save}
            onSaved={(p) => toast.success(`Saved ${p.name}`)}
          />
        </>
      )}
    </>
  )
}

export function NewPolicyPage() {
  const save = policies.useSaveTask()
  const navigate = useNavigate()
  return (
    <>
      <BackLink to="/policies">Policies</BackLink>
      <PageHeader
        icon={CalendarCheckIcon}
        tone="warning"
        title="New policy"
        description="Restart, stop or start servers or whole nodes at set times, or run console commands."
      />
      <PolicyForm
        initial={emptyPolicy}
        submitLabel="Create policy"
        save={save}
        onSaved={(p) => {
          toast.success(`Created ${p.name}`)
          void navigate({ to: "/policies", ignoreBlocker: true })
        }}
      />
    </>
  )
}

function PolicyForm({
  initial,
  submitLabel,
  save,
  onSaved,
}: {
  initial: TaskInput<PolicySettings>
  submitLabel: string
  save: ReturnType<typeof policies.useSaveTask>
  onSaved: (policy: { name: string }) => void
}) {
  // Kept as typed, so that a comma can be entered before the next number.
  const [warnings, setWarnings] = useState(initial.settings.warnings.join(", "))
  return (
    <TaskForm
      initial={initial}
      noun="policy"
      submitLabel={submitLabel}
      pending={save.isPending}
      error={save.error}
      onSubmit={(input) => save.mutate(input, { onSuccess: onSaved })}
    >
      {(settings, set) => (
        <FormSection title="Action" description="What happens at the scheduled times.">
          <RadioGroup
            value={settings.action}
            onValueChange={(action) => set({ action: action as PolicyAction })}
            aria-label="Action"
            className="gap-3 sm:grid-cols-2"
          >
            {Object.entries(actions).map(([action, { label, description, icon: Icon }]) => (
              <FieldLabel key={action} htmlFor={`action-${action}`}>
                <Field orientation="horizontal" className="items-start">
                  <Icon className="mt-0.5 size-5 shrink-0 text-warning" weight="duotone" />
                  <FieldContent>
                    <FieldTitle>{label}</FieldTitle>
                    <FieldDescription>{description}</FieldDescription>
                  </FieldContent>
                  <RadioGroupItem id={`action-${action}`} value={action} />
                </Field>
              </FieldLabel>
            ))}
          </RadioGroup>
          {warns(settings.action) && (
            <div className="grid gap-4 sm:grid-cols-[12rem_1fr]">
              <Field>
                <FieldLabel htmlFor="policy-warnings">Warn the players</FieldLabel>
                <Input
                  id="policy-warnings"
                  inputMode="numeric"
                  placeholder="10, 5, 1"
                  className="font-mono"
                  value={warnings}
                  onChange={(e) => {
                    setWarnings(e.target.value)
                    set({ warnings: e.target.value.split(/[\s,]+/).flatMap((m) => (/^\d+$/.test(m) ? [Number(m)] : [])) })
                  }}
                />
                <FieldDescription>Minutes before, up to 60.</FieldDescription>
              </Field>
              <Field>
                <FieldLabel htmlFor="policy-message">Warning</FieldLabel>
                <Input
                  id="policy-message"
                  maxLength={200}
                  placeholder={`The server ${settings.action === "restart" ? "restarts" : "stops"} in {minutes} min.`}
                  value={settings.message}
                  onChange={(e) => set({ message: e.target.value })}
                />
                <FieldDescription>Shown in the chat; {"{minutes}"} becomes the minutes left. Proxies get no warning.</FieldDescription>
              </Field>
            </div>
          )}
          {settings.action === "command" && (
            <Field>
              <FieldLabel htmlFor="policy-command">Command</FieldLabel>
              <Input
                id="policy-command"
                required
                maxLength={1000}
                placeholder="say Vote for us!"
                className="font-mono"
                value={settings.command}
                onChange={(e) => set({ command: e.target.value })}
              />
              <FieldDescription>A single console command, without a slash. Proxies don't accept console commands yet.</FieldDescription>
            </Field>
          )}
        </FormSection>
      )}
    </TaskForm>
  )
}
