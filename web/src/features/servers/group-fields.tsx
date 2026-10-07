import { PlusIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Field, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

// An example in an empty field, which needs no translation.
const exampleTag = "lobby"

/** Adds the servers with a tag, e.g. as targets of a file set; known tags are suggested. */
export function TagField({ tags, onAdd }: { tags: string[]; onAdd: (tag: string) => void }) {
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

/** Adds servers of a network: those of the roles, e.g. its game servers or its proxy, the first role by default. */
export function NetworkField<R extends string>({
  networks,
  roles,
  onAdd,
}: {
  networks: { id: string; name: string }[]
  roles: { value: R; label: string }[]
  onAdd: (network: string, role: R) => void
}) {
  const [network, setNetwork] = useState("")
  const [role, setRole] = useState(roles[0].value)
  return (
    <Field>
      <FieldLabel htmlFor="target-network">{t("Servers of a network")}</FieldLabel>
      <div className="flex max-w-xl flex-wrap gap-2">
        <Select value={network} onValueChange={setNetwork}>
          <SelectTrigger id="target-network" className="min-w-48 flex-1">
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
        <Select value={role} onValueChange={(r) => setRole(r as R)}>
          <SelectTrigger aria-label={t("Which servers of the network")} className="w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {roles.map((r) => (
              <SelectItem key={r.value} value={r.value}>
                {r.label}
              </SelectItem>
            ))}
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
