import { ClockIcon, CubeIcon, LockIcon, ShieldCheckIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { useBlocker } from "@tanstack/react-router"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { ErrorCallout } from "@/components/callout"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { FormSection } from "@/components/form-section"
import { StatCard } from "@/components/stat-card"
import { Button } from "@/components/ui/button"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { limitsForm, limitsOf } from "@/features/nodes/limits"
import { LimitsFields } from "@/features/nodes/limits-fields"
import { formatDate, formatDateTime, formatDuration } from "@/lib/format"
import { type Master, type MasterSettings, type SettingsView, settingsQuery, useUpdateSettings } from "./api"

/** The General tab: the running master and its settings. */
export function GeneralSettingsPage() {
  const { data, isPending, error } = useQuery(settingsQuery)
  if (isPending) return <Skeleton className="h-96 rounded-2xl" />
  if (error) return <ErrorCallout error={error} />
  return (
    <>
      <MasterFacts master={data.master} effectiveEnrollAddr={data.settings.enrollAddr || data.master.enrollAddr} />
      {/* Remounting on save resets the form to what the master stored. */}
      <SettingsForm key={JSON.stringify(data.settings)} view={data} />
    </>
  )
}

function MasterFacts({ master, effectiveEnrollAddr }: { master: Master; effectiveEnrollAddr: string }) {
  const [openedAt] = useState(Date.now)
  const details: [string, string][] = [
    ["Enrollment endpoint", master.enrollListenAddr],
    ["Join tokens connect to", effectiveEnrollAddr],
    ["CA fingerprint (SHA-256)", master.caFingerprint],
  ]
  return (
    <>
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard icon={CubeIcon} tone="info" label="Master" value={master.version}>
          mcsm-master
        </StatCard>
        <StatCard icon={ClockIcon} tone="success" label="Running for" value={formatDuration(openedAt - Date.parse(master.startedAt))}>
          since {formatDateTime(master.startedAt)}
        </StatCard>
        <StatCard icon={LockIcon} tone="violet" label="Panel" value={master.panelTls ? "HTTPS" : "Reverse proxy"}>
          <span className="font-mono">{master.panelAddr}</span>
        </StatCard>
        <StatCard icon={ShieldCheckIcon} tone="warning" label="Certificate valid until" value={formatDate(master.certificateExpiresAt)}>
          Renewed automatically
        </StatCard>
      </div>
      <dl className="mt-4 surface grid gap-x-6 gap-y-4 rounded-xl px-5 py-4 md:grid-cols-[auto_auto_1fr]">
        {details.map(([term, value]) => (
          <div key={term} className="min-w-0">
            <dt className="text-xs text-muted-foreground">{term}</dt>
            <dd className="mt-0.5 font-mono text-sm font-medium break-all">{value}</dd>
          </div>
        ))}
      </dl>
    </>
  )
}

function formOf(s: MasterSettings) {
  const { enrollAddr, sessionHours, joinTokenMinutes } = s
  return { enrollAddr, sessionHours, joinTokenMinutes, nodeDefaults: limitsForm(s.nodeDefaults) }
}

function SettingsForm({ view: { settings, master } }: { view: SettingsView }) {
  const editable = useAccess().can("settings.edit")
  const initial = formOf(settings)
  const [form, setForm] = useState(initial)
  const update = useUpdateSettings()
  const dirty = JSON.stringify(form) !== JSON.stringify(initial)
  const blocker = useBlocker({ shouldBlockFn: () => dirty && !update.isPending, enableBeforeUnload: () => dirty, withResolver: true })
  const set = (change: Partial<typeof form>) => setForm({ ...form, ...change })

  function submit(event: FormEvent) {
    event.preventDefault()
    toast.promise(update.mutateAsync({ ...form, nodeDefaults: limitsOf(form.nodeDefaults) }), {
      loading: "Saving…",
      success: "Saved the settings",
      error: (e: Error) => e.message,
    })
  }

  return (
    <form onSubmit={submit} className="mt-10 surface rounded-2xl px-5 sm:px-8">
      {/* Without the permission to change them, the settings are only shown. */}
      <fieldset disabled={!editable} className="contents">
        <FormSection title="Enrollment" description="How new agents reach the master with their join token.">
          <Field>
            <FieldLabel htmlFor="settings-enroll-addr">Enrollment address</FieldLabel>
            <Input
              id="settings-enroll-addr"
              className="font-mono"
              placeholder={master.enrollAddr}
              maxLength={261}
              value={form.enrollAddr}
              onChange={(e) => set({ enrollAddr: e.target.value })}
            />
            <FieldDescription>
              Host and port that join tokens tell agents to connect to, e.g. after the master moved to another domain. Leave it empty to use{" "}
              <span className="font-mono">{master.enrollAddr}</span> from the command line.
            </FieldDescription>
          </Field>
          <Field>
            <FieldLabel htmlFor="settings-join-token">Join tokens are valid for</FieldLabel>
            <NumberInput
              id="settings-join-token"
              min={5}
              max={1440}
              unit="minutes"
              value={form.joinTokenMinutes}
              onChange={(joinTokenMinutes) => set({ joinTokenMinutes })}
            />
            <FieldDescription>From 5 minutes to a day. Each token works only once.</FieldDescription>
          </Field>
        </FormSection>

        <FormSection title="Sign-in" description="Sessions of administrators in this panel.">
          <Field>
            <FieldLabel htmlFor="settings-session">Sessions last</FieldLabel>
            <NumberInput
              id="settings-session"
              min={1}
              max={168}
              unit="hours"
              value={form.sessionHours}
              onChange={(sessionHours) => set({ sessionHours })}
            />
            <FieldDescription>Up to a week. Applies from the next sign-in; shorter sessions are safer.</FieldDescription>
          </Field>
        </FormSection>

        <FormSection title="New nodes" description="Limits that nodes get when they are added. Change them for each node in its settings.">
          <LimitsFields
            id="settings-node-defaults"
            form={form.nodeDefaults}
            onChange={(change) => set({ nodeDefaults: { ...form.nodeDefaults, ...change } })}
          />
        </FormSection>
      </fieldset>
      <div
        className="-mx-5 flex flex-wrap-reverse items-center justify-end gap-x-6 gap-y-3 rounded-b-2xl bg-muted/50 px-5 py-4 sm:-mx-8 sm:px-8"
        hidden={!editable}
      >
        <p className="text-sm text-muted-foreground">Changes apply right away, without restarting the master.</p>
        <Button type="submit" disabled={!dirty || update.isPending}>
          {update.isPending ? "Saving…" : "Save settings"}
        </Button>
      </div>
      <ConfirmDialog
        open={blocker.status === "blocked"}
        onOpenChange={(open) => !open && blocker.reset?.()}
        title="Discard your changes?"
        description="Your changes to the settings haven't been saved."
        action="Discard changes"
        destructive
        onConfirm={() => blocker.proceed?.()}
      />
    </form>
  )
}

function NumberInput({
  id,
  min,
  max,
  unit,
  value,
  onChange,
}: {
  id: string
  min: number
  max: number
  unit: string
  value: number
  onChange: (value: number) => void
}) {
  return (
    <div className="flex items-center gap-2">
      <Input
        id={id}
        type="number"
        required
        min={min}
        max={max}
        className="font-mono sm:w-32"
        value={value}
        onChange={(e) => onChange(e.target.valueAsNumber || 0)}
      />
      <span className="text-sm text-muted-foreground">{unit}</span>
    </div>
  )
}
