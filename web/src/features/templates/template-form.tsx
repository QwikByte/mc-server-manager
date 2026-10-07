import { PlusIcon, XIcon } from "@phosphor-icons/react"
import { useBlocker } from "@tanstack/react-router"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { FormSection } from "@/components/form-section"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import type { Project } from "@/features/plugins/api"
import { PluginIcon } from "@/features/plugins/plugin-icon"
import { PluginSearch } from "@/features/plugins/plugin-search"
import { defaults, serverType, splitOptions } from "@/features/servers/server-types"
import { CpuLimitField, JavaFields, JvmOptionsField, MemoryField, RestartPolicyField, VersionField } from "@/features/servers/settings-fields"
import { EndOfLifeNotice, SoftwareOptions } from "@/features/servers/software"
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
      <FormSection title={t("General")}>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field>
            <FieldLabel htmlFor="template-name">{t("Name")}</FieldLabel>
            <Input
              id="template-name"
              required
              maxLength={64}
              placeholder={t("Survival")}
              value={form.name}
              onChange={(e) => set({ name: e.target.value })}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="template-type">{t("Software")}</FieldLabel>
            <Select
              value={form.type}
              onValueChange={(next) =>
                set({ type: next, memoryMb: form.memoryMb === defaults(form.type).memoryMb ? defaults(next).memoryMb : form.memoryMb })
              }
            >
              <SelectTrigger id="template-type" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SoftwareOptions chosen={initial.type} />
              </SelectContent>
            </Select>
          </Field>
        </div>
        <EndOfLifeNotice type={form.type} />
        <Field>
          <FieldLabel htmlFor="template-description">{t("Description")}</FieldLabel>
          <Textarea
            id="template-description"
            rows={2}
            maxLength={500}
            placeholder={t("Survival with permissions and an economy")}
            value={form.description}
            onChange={(e) => set({ description: e.target.value })}
          />
        </Field>
        <div className="grid gap-4 sm:grid-cols-2">
          {!type.proxy && (
            <VersionField
              id="template-version"
              value={form.version === "LATEST" ? "" : form.version}
              onChange={(version) => set({ version: version.trim() || "LATEST" })}
            />
          )}
          <MemoryField id="template-memory" value={form.memoryMb} onChange={(memoryMb) => set({ memoryMb })} />
        </div>
      </FormSection>

      <FormSection title={t("Startup")}>
        <RestartPolicyField value={form.restartPolicy} onChange={(restartPolicy) => set({ restartPolicy })} />
      </FormSection>

      <FormSection title={t("Java")}>
        {!type.proxy && <JavaFields java={form.java} aikarFlags={form.aikarFlags} onChange={set} />}
        <JvmOptionsField value={form.jvmOptions} onChange={(jvmOptions) => set({ jvmOptions })} />
        <CpuLimitField value={form.cpuLimit} onChange={(cpuLimit) => set({ cpuLimit })} />
      </FormSection>

      {!type.proxy && (
        // i18next-instrument-ignore-next-line: the name of the file
        <FormSection title="server.properties">
          <Field>
            <FieldLabel htmlFor="template-properties">{t("Properties")}</FieldLabel>
            <Textarea
              id="template-properties"
              rows={8}
              className="font-mono"
              // i18next-instrument-ignore-next-line: an example of what to enter
              placeholder={"difficulty=hard\nmax-players=50\nmotd=Welcome!"}
              value={form.properties}
              onChange={(e) => set({ properties: e.target.value })}
            />
            <FieldDescription>
              {t("One property per line as key=value. The port, address and RCON are set by the manager and can't be changed here.")}
            </FieldDescription>
          </Field>
        </FormSection>
      )}

      {type.addons && (
        <FormSection title={type.addons.kind === "mods" ? t("Mods") : t("Plugins")}>
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
                    aria-label={t("Remove {{name}}", { name: p.title })}
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
          {pending ? t("Saving…") : submitLabel}
        </Button>
      </div>
      <ConfirmDialog
        open={blocker.status === "blocked"}
        onOpenChange={(open) => !open && blocker.reset?.()}
        title={t("Discard your changes?")}
        description={t("Your changes to the template haven't been saved.")}
        action={t("Discard changes")}
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
  kind: "plugins" | "mods"
  chosen: Project[]
  onAdd: (p: Project) => void
}) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button type="button" variant="outline" className="w-fit">
          <PlusIcon />
          {kind === "mods" ? t("Add mods") : t("Add plugins")}
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{kind === "mods" ? t("Add mods to the template") : t("Add plugins to the template")}</DialogTitle>
          <DialogDescription>
            {kind === "mods"
              ? t("Only mods for {{type}} are shown. What they require is installed with them.", { type: serverType(type).label })
              : t("Only plugins for {{type}} are shown. What they require is installed with them.", { type: serverType(type).label })}
          </DialogDescription>
        </DialogHeader>
        <PluginSearch
          type={type}
          version={version}
          autoFocus
          action={(hit) =>
            chosen.some((p) => p.id === hit.id) ? (
              <Pill tone="success">{t("Added")}</Pill>
            ) : (
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => onAdd({ id: hit.id, slug: hit.slug, title: hit.title, icon: hit.icon })}
              >
                <PlusIcon />
                {t("Add")}
              </Button>
            )
          }
        />
      </DialogContent>
    </Dialog>
  )
}
