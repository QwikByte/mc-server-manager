import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useBlocker } from "@tanstack/react-router"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { FormSection } from "@/components/form-section"
import { Button } from "@/components/ui/button"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { nodeQuery } from "@/features/nodes/api"
import { type Server, type ServerSettings, useServer, useUpdateServer } from "./api"
import { serverType, splitOptions } from "./server-types"
import { CpuLimitField, JavaFields, JvmOptionsField, MemoryField, RestartPolicyField } from "./settings-fields"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId/settings")

/** The Settings tab of a server. */
export function SettingsPage() {
  const { nodeId, serverId } = route.useParams()
  const { server } = useServer(nodeId, serverId)
  // Remounting on save resets the form to what the agent applied.
  return server ? <SettingsForm key={JSON.stringify(settingsOf(server))} nodeId={nodeId} server={server} /> : null
}

function settingsOf(s: Server): ServerSettings {
  const { name, version, memoryMb, port, java, restartPolicy, aikarFlags, jvmOptions, cpuLimit } = s
  return { name, version, memoryMb, port, java, restartPolicy, aikarFlags, jvmOptions, cpuLimit }
}

function SettingsForm({ nodeId, server }: { nodeId: string; server: Server }) {
  const initial = settingsOf(server)
  const [form, setForm] = useState({ ...initial, jvmOptions: initial.jvmOptions.join("\n") })
  const update = useUpdateServer(nodeId, server.id)
  const { data: node } = useQuery(nodeQuery(nodeId))
  const game = !serverType(server.type).proxy
  const settings: ServerSettings = { ...form, jvmOptions: splitOptions(form.jvmOptions) }
  const dirty = JSON.stringify(settings) !== JSON.stringify(initial)
  const blocker = useBlocker({ shouldBlockFn: () => dirty && !update.isPending, enableBeforeUnload: () => dirty, withResolver: true })
  const set = (change: Partial<typeof form>) => setForm({ ...form, ...change })

  function submit(event: FormEvent) {
    event.preventDefault()
    toast.promise(update.mutateAsync(settings), {
      loading: server.state === "stopped" ? "Saving…" : `Saving and restarting ${server.name}…`,
      success: `Saved the settings of ${form.name}`,
      error: (e: Error) => e.message,
    })
  }

  return (
    <form onSubmit={submit} className="surface rounded-2xl px-5 sm:px-8">
      <FormSection title="General" description="Name, version and resources of the server.">
        <Field>
          <FieldLabel htmlFor="settings-name">Name</FieldLabel>
          <Input id="settings-name" required maxLength={32} value={form.name} onChange={(e) => set({ name: e.target.value })} />
        </Field>
        <div className="grid gap-4 sm:grid-cols-3">
          {game && (
            <Field>
              <FieldLabel htmlFor="settings-version">Minecraft version</FieldLabel>
              <Input
                id="settings-version"
                placeholder="Latest"
                value={form.version === "LATEST" ? "" : form.version}
                onChange={(e) => set({ version: e.target.value.trim() || "LATEST" })}
              />
            </Field>
          )}
          <MemoryField id="settings-memory" value={form.memoryMb} onChange={(memoryMb) => set({ memoryMb })} />
          <Field>
            <FieldLabel htmlFor="settings-port">Port</FieldLabel>
            <Input
              id="settings-port"
              type="number"
              min={1024}
              max={65535}
              required
              className="font-mono"
              value={form.port}
              onChange={(e) => set({ port: e.target.valueAsNumber || 0 })}
            />
          </Field>
        </div>
        {game && form.version !== initial.version && (
          <FieldDescription>Worlds can't be opened by older Minecraft versions. Make a backup before you downgrade.</FieldDescription>
        )}
      </FormSection>

      <FormSection title="Starting" description="When the server starts on its own.">
        <RestartPolicyField value={form.restartPolicy} onChange={(restartPolicy) => set({ restartPolicy })} />
      </FormSection>

      <FormSection title="Java" description="The Java runtime and the options it starts with.">
        {game && <JavaFields java={form.java} aikarFlags={form.aikarFlags} onChange={set} />}
        <JvmOptionsField value={form.jvmOptions} onChange={(jvmOptions) => set({ jvmOptions })} />
      </FormSection>

      <FormSection title="Resources" description="Limits and where the data is kept.">
        <CpuLimitField value={form.cpuLimit} onChange={(cpuLimit) => set({ cpuLimit })} cpus={node?.info?.cpuCount} />
        <p className="text-sm text-muted-foreground">
          The data is kept in the storage location <span className="font-medium text-foreground">{server.storage}</span>.
        </p>
      </FormSection>

      <div className="-mx-5 flex flex-wrap-reverse items-center justify-end gap-x-6 gap-y-3 rounded-b-2xl bg-muted/50 px-5 py-4 sm:-mx-8 sm:px-8">
        <p className="text-sm text-muted-foreground">
          The server's container is created again with the same data{server.state === "stopped" ? "." : ", so the server restarts."}
        </p>
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
