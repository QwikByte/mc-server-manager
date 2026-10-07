import { CalendarCheckIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { useState } from "react"
import { toast } from "sonner"
import { BackLink } from "@/components/back-link"
import { ErrorCallout } from "@/components/callout"
import { FormSection } from "@/components/form-section"
import { PageHeader } from "@/components/page-header"
import { usePageName } from "@/components/page-title"
import { Segmented } from "@/components/segmented"
import { Field, FieldContent, FieldDescription, FieldLabel, FieldTitle } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { useAccess } from "@/features/access/use-access"
import type { TaskInput } from "@/features/schedules/api"
import { TaskForm } from "@/features/schedules/task-form"
import { actions, emptyPolicy, type PolicyAction, type PolicySettings, policies, warns } from "./api"

const route = getRouteApi("/_app/policies/$policyId")

/** The usual numbers of servers that restart at a time, and the one a schedule has. */
const batches = (rolling: number) =>
  [...new Set([1, 2, 5, 10, rolling])].sort((a, b) => a - b).map((b) => ({ value: String(b), label: String(b) }))

export function PolicyPage() {
  const manage = useAccess().can("policies.manage")
  const { policyId } = route.useParams()
  const { data: policy, isPending, error } = useQuery(policies.taskQuery(policyId))
  usePageName(policy?.name)
  const save = policies.useSaveTask(policyId)
  return (
    <>
      <BackLink to="/policies">{t("Schedules")}</BackLink>
      {isPending ? (
        <Skeleton className="h-96 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : (
        <>
          <PageHeader icon={CalendarCheckIcon} tone="warning" title={policy.name} />
          {/* Without the permission to manage policies, the policy is only shown. */}
          <fieldset disabled={!manage} className="contents">
            <PolicyForm
              // Remounting on save resets the form to what was stored.
              key={JSON.stringify(policy)}
              initial={{
                name: policy.name,
                enabled: policy.enabled,
                schedule: policy.schedule,
                targets: policy.targets,
                settings: policy.settings,
              }}
              submitLabel={t("Save schedule")}
              save={save}
              onSaved={(p) => toast.success(t("Saved {{name}}", { name: p.name }))}
            />
          </fieldset>
        </>
      )}
    </>
  )
}

export function NewPolicyPage() {
  const save = policies.useSaveTask()
  usePageName(t("New schedule"))
  const navigate = useNavigate()
  return (
    <>
      <BackLink to="/policies">{t("Schedules")}</BackLink>
      <PageHeader icon={CalendarCheckIcon} tone="warning" title={t("New schedule")} />
      <PolicyForm
        initial={emptyPolicy}
        submitLabel={t("Create schedule")}
        save={save}
        onSaved={(p) => {
          toast.success(t("Created {{name}}", { name: p.name }))
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
      kind="policy"
      submitLabel={submitLabel}
      pending={save.isPending}
      error={save.error}
      onSubmit={(input) => save.mutate(input, { onSuccess: onSaved })}
    >
      {(settings, set) => (
        <FormSection title={t("Action")}>
          <RadioGroup
            value={settings.action}
            onValueChange={(action) => set({ action: action as PolicyAction })}
            aria-label={t("Action")}
            className="gap-3 sm:grid-cols-2"
          >
            {Object.entries(actions).map(([action, { label, description, icon: Icon }]) => (
              <FieldLabel key={action} htmlFor={`action-${action}`}>
                <Field orientation="horizontal" className="items-start">
                  <Icon className="mt-0.5 size-5 shrink-0 text-warning" weight="duotone" />
                  <FieldContent>
                    <FieldTitle>{t(label)}</FieldTitle>
                    <FieldDescription>{t(description)}</FieldDescription>
                  </FieldContent>
                  <RadioGroupItem id={`action-${action}`} value={action} />
                </Field>
              </FieldLabel>
            ))}
          </RadioGroup>
          {warns(settings.action) && (
            <div className="grid gap-4 sm:grid-cols-[12rem_1fr]">
              <Field>
                <FieldLabel htmlFor="policy-warnings">{t("Warn the players")}</FieldLabel>
                <Input
                  id="policy-warnings"
                  inputMode="numeric"
                  // i18next-instrument-ignore-next-line: an example of what to enter
                  placeholder="10, 5, 1"
                  className="font-mono"
                  value={warnings}
                  onChange={(e) => {
                    setWarnings(e.target.value)
                    set({ warnings: e.target.value.split(/[\s,]+/).flatMap((m) => (/^\d+$/.test(m) ? [Number(m)] : [])) })
                  }}
                />
                <FieldDescription>{t("Minutes before, up to 60.")}</FieldDescription>
              </Field>
              <Field>
                <FieldLabel htmlFor="policy-message">{t("Warning")}</FieldLabel>
                <Input
                  id="policy-message"
                  maxLength={200}
                  placeholder={
                    settings.action === "restart" ? t("The server restarts in {minutes} min.") : t("The server stops in {minutes} min.")
                  }
                  value={settings.message}
                  onChange={(e) => set({ message: e.target.value })}
                />
                <FieldDescription>{t("Shown in the chat; {minutes} becomes the minutes left. Proxies get no warning.")}</FieldDescription>
              </Field>
            </div>
          )}
          {settings.action === "restart" && (
            <div className="grid gap-4">
              <Field orientation="horizontal">
                <Switch id="policy-rolling" checked={!!settings.rolling} onCheckedChange={(on) => set({ rolling: on ? 1 : 0 })} />
                <FieldContent>
                  <FieldLabel htmlFor="policy-rolling">{t("Server by server in networks")}</FieldLabel>
                  <FieldDescription>
                    {t(
                      "After the warnings, the game servers of networks restart a few at a time, and their players move to another server of the network first. Their proxies restart after them, other servers at once.",
                    )}
                  </FieldDescription>
                </FieldContent>
              </Field>
              {!!settings.rolling && (
                <div className="flex flex-wrap items-center gap-3 text-sm">
                  <span>{t("Servers at a time")}</span>
                  <Segmented
                    label={t("Servers at a time")}
                    value={String(settings.rolling)}
                    onChange={(batch) => set({ rolling: Number(batch) })}
                    options={batches(settings.rolling)}
                  />
                </div>
              )}
            </div>
          )}
          {settings.action === "command" && (
            <Field>
              <FieldLabel htmlFor="policy-command">{t("Command")}</FieldLabel>
              <Input
                id="policy-command"
                required
                maxLength={1000}
                placeholder={t("say Vote for us!")}
                className="font-mono"
                value={settings.command}
                onChange={(e) => set({ command: e.target.value })}
              />
              <FieldDescription>
                {t("A single console command, without a slash. It goes to the game servers, not to proxies.")}
              </FieldDescription>
            </Field>
          )}
        </FormSection>
      )}
    </TaskForm>
  )
}
