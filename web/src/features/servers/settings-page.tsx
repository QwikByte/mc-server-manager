import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useBlocker } from "@tanstack/react-router"
import { type FormEvent, type ReactNode, useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import { Field, FieldContent, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { nodeQuery } from "@/features/nodes/api"
import { formatMegabytes } from "@/lib/format"
import { type RestartPolicy, type Server, type ServerSettings, useServer, useUpdateServer } from "./api"
import { memoryOptionsMb, serverType } from "./server-types"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId/settings")

const javaVersions: [value: string, label: string][] = [
  ["", "Newest"],
  ["25", "Java 25"],
  ["21", "Java 21"],
  ["17", "Java 17"],
  ["11", "Java 11"],
  ["8", "Java 8"],
]

const restartPolicies: [RestartPolicy, string, string][] = [
  ["always", "Always running", "Starts with the node and after a crash, unless you stopped it."],
  ["on_crash", "After a crash", "Starts again when it crashes."],
  ["never", "Only manually", "Starts only when you start it."],
]

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
  const cpus = node?.info?.cpuCount
  const game = !serverType(server.type).proxy
  const settings: ServerSettings = { ...form, jvmOptions: form.jvmOptions.split(/\s+/).filter(Boolean) }
  const dirty = JSON.stringify(settings) !== JSON.stringify(initial)
  const blocker = useBlocker({ shouldBlockFn: () => dirty && !update.isPending, enableBeforeUnload: () => dirty, withResolver: true })
  const set = <K extends keyof typeof form>(key: K, value: (typeof form)[K]) => setForm({ ...form, [key]: value })

  function submit(event: FormEvent) {
    event.preventDefault()
    toast.promise(update.mutateAsync(settings), {
      loading: server.state === "stopped" ? "Saving…" : `Saving and restarting ${server.name}…`,
      success: `Saved the settings of ${form.name}`,
      error: (e: Error) => e.message,
    })
  }

  return (
    <form onSubmit={submit} className="max-w-3xl">
      <Section title="General">
        <Field>
          <FieldLabel htmlFor="settings-name">Name</FieldLabel>
          <Input id="settings-name" required maxLength={32} value={form.name} onChange={(e) => set("name", e.target.value)} />
        </Field>
        <div className="grid gap-4 sm:grid-cols-3">
          {game && (
            <Field>
              <FieldLabel htmlFor="settings-version">Minecraft version</FieldLabel>
              <Input
                id="settings-version"
                placeholder="Latest"
                value={form.version === "LATEST" ? "" : form.version}
                onChange={(e) => set("version", e.target.value.trim() || "LATEST")}
              />
            </Field>
          )}
          <Field>
            <FieldLabel htmlFor="settings-memory">Memory</FieldLabel>
            <Select value={String(form.memoryMb)} onValueChange={(v) => set("memoryMb", Number(v))}>
              <SelectTrigger id="settings-memory" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {[...new Set([...memoryOptionsMb, initial.memoryMb])]
                  .sort((a, b) => a - b)
                  .map((mb) => (
                    <SelectItem key={mb} value={String(mb)}>
                      {formatMegabytes(mb)}
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
          </Field>
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
              onChange={(e) => set("port", e.target.valueAsNumber || 0)}
            />
          </Field>
        </div>
        {game && form.version !== initial.version && (
          <FieldDescription>Worlds can't be opened by older Minecraft versions. Make a backup before you downgrade.</FieldDescription>
        )}
      </Section>

      <Section title="Starting">
        <RadioGroup
          value={form.restartPolicy}
          onValueChange={(v) => set("restartPolicy", v as RestartPolicy)}
          aria-label="When the server starts"
        >
          {restartPolicies.map(([value, label, description]) => (
            <Field key={value} orientation="horizontal">
              <RadioGroupItem id={`restart-${value}`} value={value} />
              <FieldContent>
                <FieldLabel htmlFor={`restart-${value}`}>{label}</FieldLabel>
                <FieldDescription>{description}</FieldDescription>
              </FieldContent>
            </Field>
          ))}
        </RadioGroup>
      </Section>

      <Section title="Java">
        {game && (
          <>
            <Field>
              <FieldLabel htmlFor="settings-java">Java version</FieldLabel>
              <Select value={form.java || "newest"} onValueChange={(v) => set("java", v === "newest" ? "" : v)}>
                <SelectTrigger id="settings-java" className="w-full sm:w-64">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {javaVersions.map(([value, label]) => (
                    <SelectItem key={label} value={value || "newest"}>
                      {label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FieldDescription>
                Minecraft 1.20.5 and newer needs Java 21 or newer, 1.18 to 1.20.4 Java 17, and versions up to 1.16 run best on Java 8 or 11.
              </FieldDescription>
            </Field>
            <Field orientation="horizontal">
              <Switch id="settings-aikar" checked={form.aikarFlags} onCheckedChange={(on) => set("aikarFlags", on)} />
              <FieldContent>
                <FieldLabel htmlFor="settings-aikar">Aikar's flags</FieldLabel>
                <FieldDescription>Garbage collector tuning recommended for Paper and Purpur, which reduces lag spikes.</FieldDescription>
              </FieldContent>
            </Field>
          </>
        )}
        <Field>
          <FieldLabel htmlFor="settings-jvm">JVM options</FieldLabel>
          <Textarea
            id="settings-jvm"
            rows={3}
            className="font-mono"
            placeholder="-Dfile.encoding=UTF-8"
            value={form.jvmOptions}
            onChange={(e) => set("jvmOptions", e.target.value)}
          />
          <FieldDescription>One option per line. The memory is set above, not here.</FieldDescription>
        </Field>
      </Section>

      <Section title="Resources">
        <Field>
          <FieldLabel htmlFor="settings-cpu">CPU limit</FieldLabel>
          <Input
            id="settings-cpu"
            type="number"
            min={0}
            max={cpus}
            step={0.5}
            className="w-full font-mono sm:w-40"
            value={form.cpuLimit}
            onChange={(e) => set("cpuLimit", e.target.valueAsNumber || 0)}
          />
          <FieldDescription>
            CPU cores the server may use{cpus ? ` of the node's ${cpus}` : ""}. 0 means no limit, which suits most servers.
          </FieldDescription>
        </Field>
        <p className="text-sm text-muted-foreground">
          The data is kept in the storage location <span className="font-medium text-foreground">{server.storage}</span>.
        </p>
      </Section>

      <div className="flex flex-wrap items-center gap-4 border-t pt-6">
        <Button type="submit" disabled={!dirty || update.isPending}>
          {update.isPending ? "Saving…" : "Save settings"}
        </Button>
        <p className="text-sm text-muted-foreground">
          The server's container is created again with the same data{server.state === "stopped" ? "." : ", so the server restarts."}
        </p>
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

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="mb-10" aria-label={title}>
      <h2 className="heading mb-5 border-b pb-2 text-lg">{title}</h2>
      <FieldGroup>{children}</FieldGroup>
    </section>
  )
}
