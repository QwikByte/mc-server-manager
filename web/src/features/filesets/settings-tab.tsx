import { PlusIcon, XIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { useState } from "react"
import { FormSection } from "@/components/form-section"
import { Button } from "@/components/ui/button"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { networksQuery } from "@/features/networks/api"
import { allServersQuery } from "@/features/servers/api"
import type { SetInput, Target } from "./api"
import { targetKey, targetLabel } from "./labels"

// An example in an empty field, which needs no translation.
const exampleTag = "lobby"

/** The name, description and targets of a set. */
export function SettingsTab({ draft, onChange }: { draft: SetInput; onChange: (change: Partial<SetInput>) => void }) {
  const { data: networks } = useQuery(networksQuery)
  const { data: servers } = useQuery(allServersQuery)
  const add = (target: Target) => {
    if (!draft.targets.some((other) => targetKey(other) === targetKey(target))) onChange({ targets: [...draft.targets, target] })
  }
  const covered = (target: Target) => {
    if (target.kind === "tag") return servers?.filter((s) => s.tags.includes(target.value)).length
    const network = networks?.find((n) => n.id === target.value)
    return network && (target.role === "proxy" ? 1 : network.backends.length)
  }

  return (
    <div className="surface rounded-2xl px-5 sm:px-8">
      <FormSection title={t("Details")}>
        <Field>
          <FieldLabel htmlFor="fileset-name">{t("Name")}</FieldLabel>
          <Input id="fileset-name" required maxLength={64} value={draft.name} onChange={(e) => onChange({ name: e.target.value })} />
        </Field>
        <Field>
          <FieldLabel htmlFor="fileset-description">{t("Description")}</FieldLabel>
          <Textarea id="fileset-description" maxLength={500} rows={2} value={draft.description} onChange={(e) => onChange({ description: e.target.value })} />
        </Field>
      </FormSection>
      <FormSection title={t("Targets")}>
        <FieldDescription>
          {t(
            "Applying puts the files on all servers of the targets. A server that stops being a target loses the files with secrets right away, and the others with the next apply, unless they changed on the server.",
          )}
        </FieldDescription>
        {draft.targets.length > 0 && (
          <ul className="grid gap-2">
            {draft.targets.map((target) => {
              const { icon: Icon, label } = targetLabel(target, networks)
              const count = covered(target)
              return (
                <li key={targetKey(target)} className="flex items-center gap-3 rounded-lg px-3 py-2 ring-1 ring-foreground/8">
                  <Icon className="size-4 text-muted-foreground" weight="duotone" />
                  <span className="min-w-0 flex-1 truncate text-sm font-medium">{label}</span>
                  {count !== undefined && (
                    <span className="text-xs text-muted-foreground">{t("{{count}} servers", { count, defaultValue_one: "{{count}} server" })}</span>
                  )}
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    aria-label={t("Remove {{name}}", { name: label })}
                    onClick={() => onChange({ targets: draft.targets.filter((other) => targetKey(other) !== targetKey(target)) })}
                  >
                    <XIcon />
                  </Button>
                </li>
              )
            })}
          </ul>
        )}
        <div className="grid gap-4">
          <TagField tags={[...new Set(servers?.flatMap((s) => s.tags))].sort()} onAdd={(value) => add({ kind: "tag", value })} />
          <NetworkField networks={networks ?? []} onAdd={(value, role) => add({ kind: "network", value, role })} />
        </div>
      </FormSection>
    </div>
  )
}

function TagField({ tags, onAdd }: { tags: string[]; onAdd: (tag: string) => void }) {
  const [tag, setTag] = useState("")
  function submit(event: { preventDefault: () => void }) {
    event.preventDefault()
    if (tag.trim()) onAdd(tag.trim().toLowerCase())
    setTag("")
  }
  return (
    <Field>
      <FieldLabel htmlFor="target-tag">{t("Servers with a tag")}</FieldLabel>
      {/* Not a form of its own, as the tab is inside the page's fieldset; Enter adds the tag. */}
      <div className="flex max-w-xl gap-2">
        <Input
          id="target-tag"
          list="target-tags"
          placeholder={exampleTag}
          value={tag}
          onChange={(e) => setTag(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && submit(e)}
        />
        <datalist id="target-tags">
          {tags.map((known) => (
            <option key={known} value={known} />
          ))}
        </datalist>
        <Button type="button" variant="outline" disabled={!tag.trim()} onClick={submit}>
          <PlusIcon />
          {t("Add")}
        </Button>
      </div>
    </Field>
  )
}

function NetworkField({
  networks,
  onAdd,
}: {
  networks: { id: string; name: string }[]
  onAdd: (network: string, role: "servers" | "proxy") => void
}) {
  const [network, setNetwork] = useState("")
  const [role, setRole] = useState<"servers" | "proxy">("servers")
  return (
    <Field>
      <FieldLabel htmlFor="target-network">{t("Servers of a network")}</FieldLabel>
      <div className="flex max-w-xl flex-wrap gap-2">
        <Select value={network} onValueChange={setNetwork}>
          <SelectTrigger id="target-network" className="min-w-36 flex-1">
            <SelectValue placeholder={t("Choose a network")} />
          </SelectTrigger>
          <SelectContent>
            {networks.map((n) => (
              <SelectItem key={n.id} value={n.id}>
                {n.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={role} onValueChange={(r) => setRole(r as "servers" | "proxy")}>
          <SelectTrigger aria-label={t("Which servers of the network")} className="w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="servers">{t("Game servers")}</SelectItem>
            <SelectItem value="proxy">{t("Proxy")}</SelectItem>
          </SelectContent>
        </Select>
        <Button type="button" variant="outline" disabled={!network} onClick={() => onAdd(network, role)}>
          <PlusIcon />
          {t("Add")}
        </Button>
      </div>
    </Field>
  )
}
