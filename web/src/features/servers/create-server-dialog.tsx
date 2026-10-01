import { PlusIcon } from "@phosphor-icons/react"
import { type FormEvent, useState } from "react"
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
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectSeparator, SelectTrigger, SelectValue } from "@/components/ui/select"
import { formatMegabytes } from "@/lib/format"
import { type NewServer, useCreateServer } from "./api"
import { defaults, memoryOptionsMb, serverType, serverTypes } from "./server-types"

const initial: NewServer = { name: "", type: "paper", version: "", ...defaults("paper"), acceptEula: false }

export function CreateServerDialog({ nodeId }: { nodeId: string }) {
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState(initial)
  const create = useCreateServer(nodeId)
  const proxy = serverType(form.type).proxy

  function onOpenChange(next: boolean) {
    setOpen(next)
    if (!next) {
      create.reset()
      setForm(initial)
    }
  }

  // Switching between game server and proxy updates port and memory unless they were changed.
  function changeType(type: string) {
    const before = defaults(form.type)
    const after = defaults(type)
    setForm({
      ...form,
      type,
      port: form.port === before.port ? after.port : form.port,
      memoryMb: form.memoryMb === before.memoryMb ? after.memoryMb : form.memoryMb,
    })
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    create.mutate(
      { ...form, version: proxy ? "" : form.version.trim() },
      {
        onSuccess: (server) => {
          toast.success(`Created ${server.name}`)
          onOpenChange(false)
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button>
          <PlusIcon />
          Create server
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>Create server</DialogTitle>
            <DialogDescription>The first server on a node downloads the server image, which can take a few minutes.</DialogDescription>
          </DialogHeader>
          <FieldGroup>
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
                <Select value={form.type} onValueChange={changeType}>
                  <SelectTrigger id="server-type" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectLabel>Game servers</SelectLabel>
                      {serverTypes.filter((t) => !t.proxy).map((t) => (
                        <SelectItem key={t.value} value={t.value}>
                          {t.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                    <SelectSeparator />
                    <SelectGroup>
                      <SelectLabel>Proxies for networks</SelectLabel>
                      {serverTypes.filter((t) => t.proxy).map((t) => (
                        <SelectItem key={t.value} value={t.value}>
                          {t.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
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
                    {memoryOptionsMb.map((mb) => (
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
                  value={form.port}
                  onChange={(e) => setForm({ ...form, port: e.target.valueAsNumber || 0 })}
                />
              </Field>
            </div>
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
            <Button type="submit" disabled={create.isPending}>
              {create.isPending ? "Creating…" : "Create server"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
