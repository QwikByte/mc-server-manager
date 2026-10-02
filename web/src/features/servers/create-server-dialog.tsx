import { PlusIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { useNavigate } from "@tanstack/react-router"
import { type FormEvent, type ReactElement, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { useAccess } from "@/features/access/use-access"
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
import { nodeQuery, nodesQuery } from "@/features/nodes/api"
import { type Template, templatesQuery } from "@/features/templates/api"
import { formatBytes, formatMegabytes } from "@/lib/format"
import { type NewServer, serversQuery, useCreateServer } from "./api"
import { defaults, memoryOptionsMb, serverType, serverTypes, suggestPort } from "./server-types"

// Node, port and storage stay unset until chosen, so that the suggestions apply.
type Form = Omit<NewServer, "port" | "storage"> & { port?: number; storage?: string; nodeId?: string; templateId?: string }

const none = "none"

function blank(template?: Template): Form {
  const basics = template
    ? { type: template.type, version: template.version === "LATEST" ? "" : template.version, memoryMb: template.memoryMb }
    : { type: "paper", version: "", memoryMb: defaults("paper").memoryMb }
  return { name: "", acceptEula: false, templateId: template?.id, ...basics }
}

/** What a template adds to a new server, e.g. "Java 21 · 3 properties · LuckPerms". */
function templateSummary(t: Template) {
  const count = Object.keys(t.properties).length
  return [t.java && `Java ${t.java}`, t.aikarFlags && "Aikar's flags", count && `${count} properties`, ...t.plugins.map((p) => p.title)]
    .filter(Boolean)
    .join(" · ")
}

/**
 * Creates a server, optionally from a template. The node is chosen in the dialog unless
 * given; a given template can't be changed.
 */
export function CreateServerDialog({
  nodeId: fixedNode,
  template: fixedTemplate,
  trigger,
}: {
  nodeId?: string
  template?: Template
  trigger?: ReactElement
}) {
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState(() => blank(fixedTemplate))
  const create = useCreateServer()
  const navigate = useNavigate()
  const { data: templates = [] } = useQuery({ ...templatesQuery, enabled: open && !fixedTemplate })
  const { can } = useAccess()
  const { data: allNodes = [], isPending: nodesPending } = useQuery({ ...nodesQuery, enabled: open && !fixedNode })
  const nodes = allNodes.filter((n) => can("servers.create", n.id))
  const nodeId = fixedNode ?? form.nodeId ?? nodes.find((n) => n.status === "online")?.id
  const template = fixedTemplate ?? templates.find((t) => t.id === form.templateId)
  const { data: node } = useQuery({ ...nodeQuery(nodeId ?? ""), enabled: open && !!nodeId })
  const { data: servers } = useQuery({ ...serversQuery(nodeId ?? ""), enabled: open && !!nodeId })
  const locations = node?.info?.storage ?? []
  const proxy = serverType(form.type).proxy
  const port =
    form.port ??
    suggestPort(servers?.map((s) => s.port) ?? [], defaults(form.type).port, node?.portMin ?? undefined, node?.portMax ?? undefined)
  const storage = form.storage ?? locations.find((l) => l.name === node?.defaultStorage)?.name ?? "default"

  function onOpenChange(next: boolean) {
    setOpen(next)
    if (!next) {
      create.reset()
      setForm(blank(fixedTemplate))
    }
  }

  // Switching between game server and proxy updates the memory unless it was changed.
  function changeType(type: string) {
    const before = defaults(form.type).memoryMb
    setForm({ ...form, type, memoryMb: form.memoryMb === before ? defaults(type).memoryMb : form.memoryMb })
  }

  function chooseTemplate(id: string) {
    const chosen = templates.find((t) => t.id === id)
    setForm({ ...blank(chosen), name: form.name, acceptEula: form.acceptEula, nodeId: form.nodeId, port: form.port, storage: form.storage })
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    if (!nodeId) return
    const { name, type, memoryMb, acceptEula } = form
    const server: NewServer = { name, type, memoryMb, acceptEula, port, storage, version: proxy ? "" : form.version.trim() }
    if (template) {
      const { java, restartPolicy, aikarFlags, jvmOptions, cpuLimit, properties } = template
      Object.assign(server, { java, restartPolicy, aikarFlags, jvmOptions, cpuLimit, properties })
    }
    create.mutate(
      { nodeId, server, plugins: template?.plugins.map((p) => p.id) },
      {
        onSuccess: ({ server: created, pluginError }) => {
          if (pluginError) toast.warning(`Created ${created.name}, but its plugins couldn't be installed: ${pluginError}`)
          else toast.success(`Created ${created.name}`)
          onOpenChange(false)
          if (!fixedNode) void navigate({ to: "/nodes/$nodeId/servers/$serverId", params: { nodeId, serverId: created.id } })
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        {trigger ?? (
          <Button>
            <PlusIcon />
            Create server
          </Button>
        )}
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{fixedTemplate ? `Create server from ${fixedTemplate.name}` : "Create server"}</DialogTitle>
            <DialogDescription>The first server on a node downloads the server image, which can take a few minutes.</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            {!fixedTemplate && templates.length > 0 && (
              <Field>
                <FieldLabel htmlFor="server-template">Template</FieldLabel>
                <Select value={form.templateId ?? none} onValueChange={(id) => id && chooseTemplate(id)}>
                  <SelectTrigger id="server-template" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={none}>No template</SelectItem>
                    {templates.map((t) => (
                      <SelectItem key={t.id} value={t.id}>
                        {t.name}
                        <span className="text-muted-foreground">{serverType(t.type).label}</span>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
            )}
            {template && templateSummary(template) && <FieldDescription>From the template: {templateSummary(template)}</FieldDescription>}
            {!fixedNode && (
              <Field>
                <FieldLabel htmlFor="server-node">Node</FieldLabel>
                {/* Radix reports "" while the options of a new value load; that is no choice. */}
                <Select
                  value={nodeId ?? ""}
                  onValueChange={(id) => id && setForm({ ...form, nodeId: id, port: undefined, storage: undefined })}
                >
                  <SelectTrigger id="server-node" className="w-full">
                    <SelectValue placeholder={nodesPending ? "Loading nodes…" : "No node is online"} />
                  </SelectTrigger>
                  <SelectContent>
                    {nodes.map((n) => (
                      <SelectItem key={n.id} value={n.id} disabled={n.status !== "online"}>
                        {n.name}
                        <span className="text-muted-foreground">{n.status === "online" ? n.address : n.status}</span>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
            )}
            <Field>
              <FieldLabel htmlFor="server-name">Name</FieldLabel>
              <Input
                id="server-name"
                placeholder="Lobby"
                required
                maxLength={32}
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </Field>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="server-type">Software</FieldLabel>
                <Select value={form.type} onValueChange={changeType} disabled={!!template}>
                  <SelectTrigger id="server-type" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {[false, true].map((isProxy) => (
                      <SelectGroup key={String(isProxy)}>
                        {isProxy && <SelectSeparator />}
                        <SelectLabel>{isProxy ? "Proxies for networks" : "Game servers"}</SelectLabel>
                        {serverTypes
                          .filter((t) => t.proxy === isProxy)
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
              {!proxy && (
                <Field>
                  <FieldLabel htmlFor="server-version">Minecraft version</FieldLabel>
                  <Input
                    id="server-version"
                    placeholder="Latest"
                    value={form.version}
                    onChange={(e) => setForm({ ...form, version: e.target.value })}
                  />
                </Field>
              )}
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="server-memory">Memory</FieldLabel>
                <Select value={String(form.memoryMb)} onValueChange={(v) => setForm({ ...form, memoryMb: Number(v) })}>
                  <SelectTrigger id="server-memory" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {[...new Set([...memoryOptionsMb, form.memoryMb])]
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
                <FieldLabel htmlFor="server-port">Port</FieldLabel>
                <Input
                  id="server-port"
                  type="number"
                  min={1024}
                  max={65535}
                  required
                  className="font-mono"
                  value={port}
                  onChange={(e) => setForm({ ...form, port: e.target.valueAsNumber || 0 })}
                />
              </Field>
            </div>
            {locations.length > 1 && (
              <Field>
                <FieldLabel htmlFor="server-storage">Storage</FieldLabel>
                <Select value={storage} onValueChange={(storage) => setForm({ ...form, storage })}>
                  <SelectTrigger id="server-storage" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {locations.map((l) => (
                      <SelectItem key={l.name} value={l.name}>
                        {l.name}
                        <span className="text-muted-foreground">{formatBytes(l.freeBytes)} free</span>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
            )}
            <Field orientation="horizontal">
              <Checkbox id="server-eula" checked={form.acceptEula} onCheckedChange={(v) => setForm({ ...form, acceptEula: v === true })} />
              <FieldLabel htmlFor="server-eula" className="font-normal">
                <span>
                  I accept the{" "}
                  <a href="https://aka.ms/MinecraftEULA" target="_blank" rel="noreferrer" className="underline underline-offset-4">
                    Minecraft EULA
                  </a>
                </span>
              </FieldLabel>
            </Field>
            {proxy && <FieldDescription>Proxies always run the latest release of their software.</FieldDescription>}
            {create.error && <FieldError>{create.error.message}</FieldError>}
          </FieldGroup>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">Cancel</Button>
            </DialogClose>
            <Button type="submit" disabled={create.isPending || !nodeId}>
              {create.isPending ? (template?.plugins.length ? "Creating and installing…" : "Creating…") : "Create server"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
