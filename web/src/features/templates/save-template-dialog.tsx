import { useQuery } from "@tanstack/react-query"
import { useNavigate } from "@tanstack/react-router"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { pluginsQuery } from "@/features/plugins/api"
import { propertiesQuery } from "@/features/properties/api"
import type { Server } from "@/features/servers/api"
import { displayVersion, serverType } from "@/features/servers/server-types"
import { formatMegabytes } from "@/lib/format"
import { useSaveTemplate } from "./api"

/**
 * Saves the settings, server.properties and Modrinth plugins of a server as a template.
 * Plugin files that aren't from Modrinth, plugin configurations and worlds are left out.
 */
export function SaveTemplateDialog({
  nodeId,
  server,
  open,
  onOpenChange,
}: {
  nodeId: string
  server: Server
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const [name, setName] = useState(server.name)
  const [description, setDescription] = useState("")
  const type = serverType(server.type)
  const properties = useQuery({ ...propertiesQuery(nodeId, server.id), enabled: open && !type.proxy })
  const plugins = useQuery({ ...pluginsQuery({ nodeId, serverId: server.id }), enabled: open && !!type.addons })
  const save = useSaveTemplate()
  const navigate = useNavigate()

  const locked = new Set(properties.data?.locked.map((l) => l.key))
  const props = Object.fromEntries(Object.entries(properties.data?.properties ?? {}).filter(([key]) => !locked.has(key)))
  const projects = plugins.data?.plugins.flatMap((p) => (p.project ? [p.project.id] : [])) ?? []
  const others = (plugins.data?.plugins.length ?? 0) - projects.length
  const kind = type.addons?.kind ?? "plugins"
  const summary = [
    `${type.label}${type.proxy ? "" : ` ${displayVersion(server.version)}`} with ${formatMegabytes(server.memoryMb)} and the server's settings`,
    !type.proxy && `${Object.keys(props).length} properties of server.properties`,
    type.addons &&
      `${projects.length} ${projects.length === 1 ? kind.slice(0, -1) : kind} from Modrinth${others > 0 ? `; ${others} other files are left out` : ""}`,
  ].filter(Boolean)

  function submit(event: FormEvent) {
    event.preventDefault()
    const { version, memoryMb, java, restartPolicy, aikarFlags, jvmOptions, cpuLimit } = server
    save.mutate(
      {
        name: name.trim(),
        description,
        type: server.type,
        version,
        memoryMb,
        java,
        restartPolicy,
        aikarFlags,
        jvmOptions,
        cpuLimit,
        properties: props,
        plugins: projects,
      },
      {
        onSuccess: (t) => {
          toast.success(`Saved ${server.name} as the template ${t.name}`)
          onOpenChange(false)
          void navigate({ to: "/templates/$templateId", params: { templateId: t.id } })
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>Save as template</DialogTitle>
            <DialogDescription>
              New servers can start with the setup of {server.name}. Worlds and plugin configurations aren't included.
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="template-name">Name</FieldLabel>
              <Input id="template-name" required maxLength={64} value={name} onChange={(e) => setName(e.target.value)} />
            </Field>
            <Field>
              <FieldLabel htmlFor="template-description">Description</FieldLabel>
              <Textarea
                id="template-description"
                rows={2}
                maxLength={500}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </Field>
            <ul className="grid gap-1.5 rounded-lg bg-muted/60 px-4 py-3 text-sm">
              {summary.map((line) => (
                <li key={String(line)} className="list-inside list-disc">
                  {line}
                </li>
              ))}
            </ul>
            {(properties.error || plugins.error) && <FieldError>{(properties.error ?? plugins.error)?.message}</FieldError>}
            {save.error && <FieldError>{save.error.message}</FieldError>}
          </FieldGroup>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">Cancel</Button>
            </DialogClose>
            <Button type="submit" disabled={save.isPending || properties.isLoading || plugins.isLoading}>
              {save.isPending ? "Saving…" : "Save template"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
