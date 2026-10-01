import { PlusIcon, XIcon } from "@phosphor-icons/react"
import { useBlocker } from "@tanstack/react-router"
import { type FormEvent, useState } from "react"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { FormSection } from "@/components/form-section"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import type { Project } from "@/features/plugins/api"
import { PluginIcon } from "@/features/plugins/plugin-icon"
import { PluginSearch } from "@/features/plugins/plugin-search"
import { defaults, serverType, serverTypes, splitOptions } from "@/features/servers/server-types"
import { CpuLimitField, JavaFields, JvmOptionsField, MemoryField, RestartPolicyField } from "@/features/servers/settings-fields"
import { parseProperties, propertiesText, type TemplateDraft, type TemplateInput } from "./api"

/** Compares drafts regardless of the order of their properties. */
const snapshot = (d: TemplateDraft) => JSON.stringify({ ...d, name: d.name.trim(), properties: propertiesText(d.properties) })

/** Edits the settings, server.properties and plugins of a template. */
export function TemplateForm({
  initial,
  submitLabel,
  pending,
  error,
  onSubmit,
}: {
  initial: TemplateDraft
  submitLabel: string
  pending: boolean
  error: Error | null
  onSubmit: (input: TemplateInput) => void
}) {
  const [form, setForm] = useState({
    ...initial,
    jvmOptions: initial.jvmOptions.join("\n"),
    properties: propertiesText(initial.properties),
  })
  const type = serverType(form.type)
  const input: TemplateInput = {
    ...form,
    name: form.name.trim(),
    version: type.proxy ? "LATEST" : form.version,
    jvmOptions: splitOptions(form.jvmOptions),
    properties: type.proxy ? {} : parseProperties(form.properties),
    plugins: type.addons ? form.plugins.map((p) => p.id) : [],
  }
  const dirty = snapshot({ ...input, plugins: form.plugins }) !== snapshot(initial)
  const blocker = useBlocker({ shouldBlockFn: () => dirty && !pending, enableBeforeUnload: () => dirty, withResolver: true })
  const set = (change: Partial<typeof form>) => setForm({ ...form, ...change })

  function submit(event: FormEvent) {
    event.preventDefault()
    onSubmit(input)
  }

  return (
    <form onSubmit={submit} className="surface rounded-2xl px-5 sm:px-8">
      <FormSection title="General" description="What the template is for, and the software of its servers.">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field>
            <FieldLabel htmlFor="template-name">Name</FieldLabel>
            <Input
              id="template-name"
              required
              maxLength={64}
              placeholder="Survival"
              value={form.name}
              onChange={(e) => set({ name: e.target.value })}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="template-type">Software</FieldLabel>
            <Select
              value={form.type}
              onValueChange={(t) =>
                set({ type: t, memoryMb: form.memoryMb === defaults(form.type).memoryMb ? defaults(t).memoryMb : form.memoryMb })
              }
            >
              <SelectTrigger id="template-type" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {[false, true].map((proxy) => (
                  <SelectGroup key={String(proxy)}>
                    {proxy && <SelectSeparator />}
                    <SelectLabel>{proxy ? "Proxies for networks" : "Game servers"}</SelectLabel>
                    {serverTypes
                      .filter((t) => t.proxy === proxy)
                      .map((t) => (
                        <SelectItem key={t.value} value={t.value}>
                          {t.label}
                        </SelectItem>
                      ))}
                  </SelectGroup>
                ))}
              </SelectContent>
            </Select>
          </Field>
        </div>
        <Field>
          <FieldLabel htmlFor="template-description">Description</FieldLabel>
          <Textarea
            id="template-description"
            rows={2}
            maxLength={500}
            placeholder="Survival with permissions and an economy"
            value={form.description}
            onChange={(e) => set({ description: e.target.value })}
          />
        </Field>
        <div className="grid gap-4 sm:grid-cols-2">
          {!type.proxy && (
            <Field>
              <FieldLabel htmlFor="template-version">Minecraft version</FieldLabel>
              <Input
                id="template-version"
                placeholder="Latest"
                value={form.version === "LATEST" ? "" : form.version}
                onChange={(e) => set({ version: e.target.value.trim() || "LATEST" })}
              />
            </Field>
          )}
          <MemoryField id="template-memory" value={form.memoryMb} onChange={(memoryMb) => set({ memoryMb })} />
        </div>
      </FormSection>

      <FormSection title="Starting" description="When the servers start on their own.">
        <RestartPolicyField value={form.restartPolicy} onChange={(restartPolicy) => set({ restartPolicy })} />
      </FormSection>

      <FormSection title="Java" description="The Java runtime and the options the servers start with.">
        {!type.proxy && <JavaFields java={form.java} aikarFlags={form.aikarFlags} onChange={set} />}
        <JvmOptionsField value={form.jvmOptions} onChange={(jvmOptions) => set({ jvmOptions })} />
        <CpuLimitField value={form.cpuLimit} onChange={(cpuLimit) => set({ cpuLimit })} />
      </FormSection>

      {!type.proxy && (
        <FormSection title="server.properties" description="Written before the first start of each server.">
          <Field>
            <FieldLabel htmlFor="template-properties">Properties</FieldLabel>
            <Textarea
              id="template-properties"
              rows={8}
              className="font-mono"
              placeholder={"difficulty=hard\nmax-players=50\nmotd=Welcome!"}
              value={form.properties}
              onChange={(e) => set({ properties: e.target.value })}
            />
            <FieldDescription>
              One property per line as key=value. The port, address and RCON are set by the manager and can't be changed here.
            </FieldDescription>
          </Field>
        </FormSection>
      )}

      {type.addons && (
        <FormSection
          title={type.addons.kind === "mods" ? "Mods" : "Plugins"}
          description="Installed from Modrinth in the newest release that suits each new server."
        >
          {form.plugins.length > 0 && (
            <ul className="flex flex-wrap gap-2">
              {form.plugins.map((p) => (
                <li key={p.id} className="flex items-center gap-2 rounded-lg bg-muted/60 py-1 pr-1 pl-1.5 text-sm font-medium">
                  <PluginIcon src={p.icon} className="size-6 rounded-md [&>svg]:size-3.5" />
                  {p.title}
                  <Button
                    type="button"
                    size="icon-xs"
                    variant="ghost"
                    aria-label={`Remove ${p.title}`}
                    onClick={() => set({ plugins: form.plugins.filter((x) => x.id !== p.id) })}
                  >
                    <XIcon />
                  </Button>
                </li>
              ))}
            </ul>
          )}
          <AddPlugins
            type={form.type}
            version={form.version}
            kind={type.addons.kind}
            chosen={form.plugins}
            onAdd={(p) => set({ plugins: [...form.plugins, p] })}
          />
        </FormSection>
      )}

      <div className="-mx-5 flex flex-wrap-reverse items-center justify-end gap-x-6 gap-y-3 rounded-b-2xl bg-muted/50 px-5 py-4 sm:-mx-8 sm:px-8">
        {error && <FieldError className="mr-auto">{error.message}</FieldError>}
        <Button type="submit" disabled={!dirty || pending}>
          {pending ? "Saving…" : submitLabel}
        </Button>
      </div>
      <ConfirmDialog
        open={blocker.status === "blocked"}
        onOpenChange={(open) => !open && blocker.reset?.()}
        title="Discard your changes?"
        description="Your changes to the template haven't been saved."
        action="Discard changes"
        destructive
        onConfirm={() => blocker.proceed?.()}
      />
    </form>
  )
}

function AddPlugins({
  type,
  version,
  kind,
  chosen,
  onAdd,
}: {
  type: string
  version: string
  kind: string
  chosen: Project[]
  onAdd: (p: Project) => void
}) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button type="button" variant="outline" className="w-fit">
          <PlusIcon />
          Add {kind}
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Add {kind} to the template</DialogTitle>
          <DialogDescription>
            Only {kind} for {serverType(type).label} are shown. What they require is installed with them.
          </DialogDescription>
        </DialogHeader>
        <PluginSearch
          type={type}
          version={version}
          autoFocus
          action={(hit) =>
            chosen.some((p) => p.id === hit.id) ? (
              <Pill tone="success">Added</Pill>
            ) : (
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => onAdd({ id: hit.id, slug: hit.slug, title: hit.title, icon: hit.icon })}
              >
                <PlusIcon />
                Add
              </Button>
            )
          }
        />
      </DialogContent>
    </Dialog>
  )
}
