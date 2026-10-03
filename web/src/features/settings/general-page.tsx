import { ArrowClockwiseIcon, ClockIcon, CubeIcon, LockIcon, ShieldCheckIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { useBlocker } from "@tanstack/react-router"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { Callout, ErrorCallout } from "@/components/callout"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { FormSection } from "@/components/form-section"
import { StatCard } from "@/components/stat-card"
import { Button } from "@/components/ui/button"
import { Field, FieldContent, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { useAccess } from "@/features/access/use-access"
import { limitsForm, limitsOf } from "@/features/nodes/limits"
import { LimitsFields } from "@/features/nodes/limits-fields"
import { UpdateCheck } from "@/features/updates/update-check"
import { formatDate, formatDateTime, formatDuration } from "@/lib/format"
import { type Master, type MasterSettings, type SettingsView, settingsQuery, useUpdateSettings } from "./api"
import { RestartButton } from "./restart-button"

/** The General tab: the running master and its settings. */
export function GeneralSettingsPage() {
  const { data, isPending, error } = useQuery(settingsQuery)
  if (isPending) return <Skeleton className="h-96 rounded-2xl" />
  if (error) return <ErrorCallout error={error} />
  return (
    <>
      {/* Remounting after a restart measures the uptime from now. */}
      <MasterFacts key={data.master.startedAt} master={data.master} settings={data.settings} />
      {/* Remounting on save resets the form to what the master stored. */}
      <SettingsForm key={JSON.stringify(data.settings)} view={data} />
    </>
  )
}

/** Where the panel listens after the next start of the master. */
const nextPanelAddr = (settings: MasterSettings, master: Master) => settings.panelAddr || master.panelDefaultAddr

/** Whether the signed-in user can restart the master from the panel. */
function useCanRestart(master: Master) {
  return useAccess().admin && master.restartable
}

function MasterFacts({ master, settings }: { master: Master; settings: MasterSettings }) {
  const [openedAt] = useState(Date.now)
  const canRestart = useCanRestart(master)
  const details: [string, string][] = [
    ["Enrollment endpoint", master.enrollListenAddr],
    ["Join tokens connect to", settings.enrollAddr || master.enrollAddr],
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
      <div className="mt-4 surface flex flex-wrap items-center gap-x-6 gap-y-4 rounded-xl px-5 py-4">
        <dl className="grid min-w-0 flex-1 gap-x-6 gap-y-4 md:grid-cols-[auto_auto_1fr]">
          {details.map(([term, value]) => (
            <div key={term} className="min-w-0">
              <dt className="text-xs text-muted-foreground">{term}</dt>
              <dd className="mt-0.5 font-mono text-sm font-medium break-all">{value}</dd>
            </div>
          ))}
        </dl>
        {canRestart && <RestartButton master={master} next={nextPanelAddr(settings, master)} />}
      </div>
    </>
  )
}

function formOf(s: MasterSettings) {
  const { panelAddr, enrollAddr, sessionHours, joinTokenMinutes, logDays, checkUpdates } = s
  return { panelAddr, enrollAddr, sessionHours, joinTokenMinutes, logDays, checkUpdates, nodeDefaults: limitsForm(s.nodeDefaults) }
}

function SettingsForm({ view: { settings, master } }: { view: SettingsView }) {
  const access = useAccess()
  const editable = access.can("settings.edit")
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
        <FormSection title="Panel" description="Where this panel can be reached. A new address applies when the master starts again.">
          <Field>
            <FieldLabel htmlFor="settings-panel-addr">Listen address</FieldLabel>
            <Input
              id="settings-panel-addr"
              className="font-mono"
              placeholder={master.panelDefaultAddr}
              maxLength={47}
              disabled={!access.admin}
              value={form.panelAddr}
              onChange={(e) => set({ panelAddr: e.target.value })}
            />
            <FieldDescription>
              IP address and port, e.g. <span className="font-mono">0.0.0.0:8080</span> for all interfaces, with a port from 1024 on. Leave it
              empty to use <span className="font-mono">{master.panelDefaultAddr}</span> from the command line, which the panel also falls back
              to if it can't listen at this address. Browsers only sign in over HTTPS, e.g. through a reverse proxy. Only administrators can
              change it, as it can open the panel to other networks.
            </FieldDescription>
          </Field>
          <PanelRestartNotice settings={settings} master={master} />
        </FormSection>

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

        <FormSection title="Log" description="What the master and its agents did and what went wrong, shown on the Logs page.">
          <Field>
            <FieldLabel htmlFor="settings-log-days">Keep entries for</FieldLabel>
            <NumberInput id="settings-log-days" min={1} max={365} unit="days" value={form.logDays} onChange={(logDays) => set({ logDays })} />
            <FieldDescription>
              Up to a year. Older entries are deleted every hour; the newest million are kept at most. Export entries to keep them longer.
            </FieldDescription>
          </Field>
        </FormSection>

        <FormSection title="Updates" description="New releases of MC Server Manager, which administrators install from the panel.">
          <Field orientation="horizontal">
            <Switch id="settings-check-updates" checked={form.checkUpdates} onCheckedChange={(checkUpdates) => set({ checkUpdates })} />
            <FieldContent>
              <FieldLabel htmlFor="settings-check-updates">Check for updates</FieldLabel>
              <FieldDescription>
                The master asks GitHub for the latest release every 6 hours. Administrators then see a notice and install it with a click.
              </FieldDescription>
            </FieldContent>
          </Field>
          {access.admin && <UpdateCheck />}
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
        <p className="text-sm text-muted-foreground">Changes apply right away, the panel's address when the master starts again.</p>
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

/** Tells where the panel listens until the master starts again, if the settings name another address. */
function PanelRestartNotice({ settings, master }: { settings: MasterSettings; master: Master }) {
  const canRestart = useCanRestart(master)
  const next = nextPanelAddr(settings, master)
  if (next === master.panelAddr) return null
  return (
    <Callout tone={master.panelAddrError ? "warning" : "info"} icon={ArrowClockwiseIcon} role="status" title="Applies after a restart">
      <p>
        The panel listens at <span className="font-mono">{master.panelAddr}</span> until the master starts again, e.g. with{" "}
        <span className="font-mono">systemctl restart mcsm-master</span> or the next update, then at <span className="font-mono">{next}</span>.
        Point a reverse proxy in front of it to the new address as well.
      </p>
      {master.panelAddrError && (
        <p className="mt-1">
          When it started last, it couldn't listen at the address from the settings: <span className="font-mono">{master.panelAddrError}</span>
        </p>
      )}
      {canRestart && (
        <div className="mt-3">
          <RestartButton master={master} next={next} label="Restart now" />
        </div>
      )}
    </Callout>
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
