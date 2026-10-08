import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { type FormEvent, type ReactNode, useState } from "react"
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
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { categories } from "@/features/logs/meta"
import { nodesQuery } from "@/features/nodes/api"
import { allServersQuery } from "@/features/servers/api"
import { type Channel, type Rule, type RuleInput, useSaveRule } from "./api"
import { kinds, ruleLevels } from "./meta"

const any = "any"

function formOf(rule: Rule | undefined, channels: Channel[]): RuleInput {
  return rule
    ? { channelId: rule.channelId, enabled: rule.enabled, level: rule.level, categories: rule.categories, nodeId: rule.nodeId, serverId: rule.serverId }
    : { channelId: channels[0]?.id ?? "", enabled: true, level: "warn", categories: [], nodeId: "", serverId: "" }
}

/** Creates a rule, or changes one: which entries of the log go to which channel. */
export function RuleDialog({ rule, channels, trigger }: { rule?: Rule; channels: Channel[]; trigger: ReactNode }) {
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState(() => formOf(rule, channels))
  const save = useSaveRule()
  const { data: nodes = [] } = useQuery({ ...nodesQuery, enabled: open })
  const { data: servers = [] } = useQuery({ ...allServersQuery, enabled: open && !!form.nodeId })
  const set = (change: Partial<RuleInput>) => setForm({ ...form, ...change })
  const toggle = (category: string, on: boolean) =>
    set({ categories: on ? [...form.categories, category] : form.categories.filter((c) => c !== category) })

  function onOpenChange(next: boolean) {
    setOpen(next)
    if (next) setForm(formOf(rule, channels))
    else save.reset()
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    save.mutate(
      { id: rule?.id, ...form },
      {
        onSuccess: () => {
          toast.success(rule ? t("Saved the rule") : t("Added the rule"))
          setOpen(false)
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent className="sm:max-w-xl">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{rule ? t("Change the rule") : t("Add a rule")}</DialogTitle>
            <DialogDescription>
              {t("New entries of the log that match the rule go to its channel. Entries that come at once go together, and a channel gets a few messages at once, then one a minute at most.")}
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="rule-channel">{t("Channel")}</FieldLabel>
                <Select value={form.channelId} onValueChange={(channelId) => set({ channelId })}>
                  <SelectTrigger id="rule-channel" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {channels.map((c) => {
                      const Icon = kinds[c.kind].icon
                      return (
                        <SelectItem key={c.id} value={c.id}>
                          <Icon weight="duotone" />
                          {c.name}
                        </SelectItem>
                      )
                    })}
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel htmlFor="rule-level">{t("Entries")}</FieldLabel>
                <Select value={form.level} onValueChange={(level: Rule["level"]) => set({ level })}>
                  <SelectTrigger id="rule-level" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {ruleLevels.map((l) => (
                      <SelectItem key={l.value} value={l.value}>
                        {t(l.label)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
            </div>
            <FieldSet>
              <FieldLegend variant="label">{t("Categories")}</FieldLegend>
              <div className="grid grid-cols-2 gap-x-4 gap-y-2.5 sm:grid-cols-3">
                {Object.entries(categories).map(([id, label]) => (
                  <label key={id} className="flex cursor-pointer items-center gap-2 text-sm">
                    <Checkbox checked={form.categories.includes(id)} onCheckedChange={(on) => toggle(id, on === true)} />
                    {t(label)}
                  </label>
                ))}
              </div>
              <FieldDescription>{t("Without any, entries of all categories match.")}</FieldDescription>
            </FieldSet>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="rule-node">{t("Node")}</FieldLabel>
                <Select value={form.nodeId || any} onValueChange={(v) => set({ nodeId: v === any ? "" : v, serverId: "" })}>
                  <SelectTrigger id="rule-node" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={any}>{t("All nodes")}</SelectItem>
                    {form.nodeId && !nodes.some((n) => n.id === form.nodeId) && <SelectItem value={form.nodeId}>{form.nodeId}</SelectItem>}
                    {nodes.map((n) => (
                      <SelectItem key={n.id} value={n.id}>
                        {n.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel htmlFor="rule-server">{t("Server")}</FieldLabel>
                <Select value={form.serverId || any} onValueChange={(v) => set({ serverId: v === any ? "" : v })} disabled={!form.nodeId}>
                  <SelectTrigger id="rule-server" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={any}>{t("All servers")}</SelectItem>
                    {form.serverId && !servers.some((s) => s.id === form.serverId) && (
                      <SelectItem value={form.serverId}>{form.serverId}</SelectItem>
                    )}
                    {servers
                      .filter((s) => s.nodeId === form.nodeId)
                      .map((s) => (
                        <SelectItem key={s.id} value={s.id}>
                          {s.name}
                        </SelectItem>
                      ))}
                  </SelectContent>
                </Select>
              </Field>
            </div>
            <Field orientation="horizontal">
              <Switch id="rule-enabled" checked={form.enabled} onCheckedChange={(enabled) => set({ enabled })} />
              <FieldLabel htmlFor="rule-enabled">{t("Send entries that match")}</FieldLabel>
            </Field>
            {save.error && <FieldError>{save.error.message}</FieldError>}
          </FieldGroup>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={save.isPending || !form.channelId}>
              {save.isPending ? t("Saving…") : t("Save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
