import { GearIcon } from "@phosphor-icons/react"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
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
import { Field, FieldContent, FieldDescription, FieldError, FieldGroup, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { formatBytes, formatMegabytes } from "@/lib/format"
import { type Node, useUpdateNode } from "./api"

/** Form state; empty strings stand for unset numbers. */
function formOf(node: Node) {
  return {
    name: node.name,
    address: node.address,
    defaultStorage: node.defaultStorage,
    portMin: node.portMin?.toString() ?? "",
    portMax: node.portMax?.toString() ?? "",
    limitMemory: node.memoryReserveMb !== null,
    memoryReserveMb: (node.memoryReserveMb ?? 1024).toString(),
  }
}

const number = (value: string) => (value.trim() === "" ? null : Number(value))

export function NodeSettingsDialog({ node }: { node: Node }) {
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState(() => formOf(node))
  const update = useUpdateNode(node.id)
  const locations = node.info?.storage ?? []
  const memoryMb = node.info?.memoryBytes ? Math.floor(node.info.memoryBytes / 1024 ** 2) : undefined
  const reserve = Number(form.memoryReserveMb) || 0
  const set = <K extends keyof typeof form>(key: K, value: (typeof form)[K]) => setForm({ ...form, [key]: value })

  function onOpenChange(next: boolean) {
    setOpen(next)
    if (next) setForm(formOf(node))
    else update.reset()
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    update.mutate(
      {
        name: form.name,
        address: form.address,
        defaultStorage: form.defaultStorage,
        portMin: number(form.portMin),
        portMax: number(form.portMax),
        memoryReserveMb: form.limitMemory ? reserve : null,
      },
      {
        onSuccess: () => {
          toast.success(`Saved the settings of ${form.name}`)
          setOpen(false)
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button variant="outline">
          <GearIcon />
          Settings
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>Settings of {node.name}</DialogTitle>
            <DialogDescription>The limits apply when servers are created or changed.</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="node-settings-name">Name</FieldLabel>
                <Input id="node-settings-name" required maxLength={64} value={form.name} onChange={(e) => set("name", e.target.value)} />
              </Field>
              <Field>
                <FieldLabel htmlFor="node-settings-address">Agent address</FieldLabel>
                <Input
                  id="node-settings-address"
                  required
                  className="font-mono"
                  value={form.address}
                  onChange={(e) => set("address", e.target.value)}
                />
              </Field>
            </div>
            {locations.length > 0 && (
              <Field>
                <FieldLabel htmlFor="node-settings-storage">Default storage</FieldLabel>
                <Select value={form.defaultStorage} onValueChange={(v) => set("defaultStorage", v)}>
                  <SelectTrigger id="node-settings-storage" className="w-full">
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
                <FieldDescription>Preselected when you create a server on this node.</FieldDescription>
              </Field>
            )}
            <FieldSet>
              <FieldLegend variant="label">Port range</FieldLegend>
              <div className="flex items-center gap-2">
                <Input
                  aria-label="First port"
                  type="number"
                  min={1024}
                  max={65535}
                  placeholder="1024"
                  className="font-mono"
                  value={form.portMin}
                  onChange={(e) => set("portMin", e.target.value)}
                />
                <span className="text-muted-foreground">–</span>
                <Input
                  aria-label="Last port"
                  type="number"
                  min={1024}
                  max={65535}
                  placeholder="65535"
                  className="font-mono"
                  value={form.portMax}
                  onChange={(e) => set("portMax", e.target.value)}
                />
              </div>
              <FieldDescription>New servers get the first free port of the range. Leave both empty to allow any port.</FieldDescription>
            </FieldSet>
            <Field orientation="horizontal">
              <Switch id="node-settings-limit" checked={form.limitMemory} onCheckedChange={(on) => set("limitMemory", on)} />
              <FieldContent>
                <FieldLabel htmlFor="node-settings-limit">Limit memory</FieldLabel>
                <FieldDescription>
                  Servers together can't get more memory than the node has, minus a reserve for the system.
                </FieldDescription>
              </FieldContent>
            </Field>
            {form.limitMemory && (
              <Field>
                <FieldLabel htmlFor="node-settings-reserve">Reserve in MB</FieldLabel>
                <Input
                  id="node-settings-reserve"
                  type="number"
                  min={0}
                  className="font-mono sm:w-40"
                  value={form.memoryReserveMb}
                  onChange={(e) => set("memoryReserveMb", e.target.value)}
                />
                <FieldDescription>
                  {memoryMb
                    ? `Servers can get up to ${formatMegabytes(Math.max(0, memoryMb - reserve))} of the node's ${formatMegabytes(memoryMb)}. `
                    : ""}
                  Java uses about a quarter more than the memory a server gets, so keep some room.
                </FieldDescription>
              </Field>
            )}
            {update.error && <FieldError>{update.error.message}</FieldError>}
          </FieldGroup>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">Cancel</Button>
            </DialogClose>
            <Button type="submit" disabled={update.isPending}>
              {update.isPending ? "Saving…" : "Save"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
