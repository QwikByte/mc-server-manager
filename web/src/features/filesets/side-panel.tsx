import { CrosshairIcon, NotePencilIcon, PlusIcon, XIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { networksQuery } from "@/features/networks/api"
import { allServersQuery } from "@/features/servers/api"
import type { FileSet, SetInput, Target } from "./api"
import { targetKey, targetLabel } from "./labels"
import { PanelSection } from "./panel-section"
import { SecretsSection } from "./secrets"

// An example in an empty field, which needs no translation.
const exampleTag = "lobby"

/** What a set is for and what it needs besides its files: its targets, secrets and details. */
export function SidePanel({ set, draft, editable, onChange }: { set: FileSet; draft: SetInput; editable: boolean; onChange: (change: Partial<SetInput>) => void }) {
  return (
    <div className="grid content-start items-start gap-4 lg:grid-cols-3 2xl:grid-cols-1">
      <TargetsSection targets={draft.targets} editable={editable} onChange={(targets) => onChange({ targets })} />
      <SecretsSection set={set} files={draft.files} editable={editable} />
      <PanelSection icon={NotePencilIcon} title={t("Details")}>
        <Field>
          <FieldLabel htmlFor="fileset-name">{t("Name")}</FieldLabel>
          <Input id="fileset-name" required maxLength={64} disabled={!editable} value={draft.name} onChange={(e) => onChange({ name: e.target.value })} />
        </Field>
        <Field>
          <FieldLabel htmlFor="fileset-description">{t("Description")}</FieldLabel>
          <Textarea
            id="fileset-description"
            maxLength={500}
            rows={3}
            disabled={!editable}
            value={draft.description}
            onChange={(e) => onChange({ description: e.target.value })}
          />
        </Field>
      </PanelSection>
    </div>
  )
}

/** The tags and networks whose servers a set is for, with how many servers each covers. */
function TargetsSection({ targets, editable, onChange }: { targets: Target[]; editable: boolean; onChange: (targets: Target[]) => void }) {
  const { data: networks } = useQuery(networksQuery)
  const { data: servers } = useQuery(allServersQuery)
  const [adding, setAdding] = useState(false)
  const add = (target: Target) => {
    if (!targets.some((other) => targetKey(other) === targetKey(target))) onChange([...targets, target])
    setAdding(false)
  }
  const covered = (target: Target) => {
    if (target.kind === "tag") return servers?.filter((s) => s.tags.includes(target.value)).length
    const network = networks?.find((n) => n.id === target.value)
    return network && (target.role === "proxy" ? 1 : network.backends.length)
  }

  return (
    <PanelSection
      icon={CrosshairIcon}
      title={t("Targets")}
      action={
        editable && (
          <Button size="icon-sm" variant="ghost" aria-label={t("Add target")} title={t("Add target")} onClick={() => setAdding(true)}>
            <PlusIcon />
          </Button>
        )
      }
    >
      {targets.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("No targets yet. Add a tag or a network, whose servers get the files when the set is applied.")}</p>
      ) : (
        <ul className="grid gap-1">
          {targets.map((target) => {
            const { icon: Icon, label } = targetLabel(target, networks)
            const count = covered(target)
            return (
              <li key={targetKey(target)} className="flex items-center gap-2 rounded-lg py-1 pr-1 pl-2 ring-1 ring-foreground/8">
                <Icon className="size-4 shrink-0 text-muted-foreground" weight="duotone" />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium">{label}</span>
                  {count !== undefined && (
                    <span className="block text-xs text-muted-foreground">{t("{{count}} servers", { count, defaultValue_one: "{{count}} server" })}</span>
                  )}
                </span>
                {editable && (
                  <Button
                    size="icon-sm"
                    variant="ghost"
                    aria-label={t("Remove {{name}}", { name: label })}
                    onClick={() => onChange(targets.filter((other) => targetKey(other) !== targetKey(target)))}
                  >
                    <XIcon />
                  </Button>
                )}
              </li>
            )
          })}
        </ul>
      )}
      {adding && (
        <Dialog open onOpenChange={setAdding}>
          <DialogContent className="sm:max-w-lg">
            <div className="grid gap-6">
              <DialogHeader>
                <DialogTitle>{t("Add target")}</DialogTitle>
                <DialogDescription>
                  {t(
                    "Applying puts the files on all servers of the targets. A server that stops being a target loses the files with secrets right away, and the others with the next apply, unless they changed on the server.",
                  )}
                </DialogDescription>
              </DialogHeader>
              <TagField tags={[...new Set(servers?.flatMap((s) => s.tags))].sort()} onAdd={(value) => add({ kind: "tag", value })} />
              <NetworkField networks={networks ?? []} onAdd={(value, role) => add({ kind: "network", value, role })} />
              <DialogFooter>
                <DialogClose asChild>
                  <Button variant="outline">{t("Cancel")}</Button>
                </DialogClose>
              </DialogFooter>
            </div>
          </DialogContent>
        </Dialog>
      )}
    </PanelSection>
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
      {/* Not a form of its own, as it may be inside one; Enter adds the tag. */}
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
