import { FolderOpenIcon, XIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { useState } from "react"
import { Trans } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Field, FieldContent, FieldDescription, FieldError, FieldLabel, FieldLegend, FieldSet, FieldTitle } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import type { ServerFiles } from "@/features/files/api"
import { PickDialog } from "@/features/files/path-picker"
import { msg } from "@/lib/i18n"
import { pathsError, type Selection } from "./api"

const choices: [key: keyof Omit<Selection, "paths">, label: string, description: string][] = [
  ["everything", msg("Everything"), msg("The whole folder of the server, including the server software.")],
  ["worlds", msg("Worlds"), msg("All worlds, also those added later.")],
  ["plugins", msg("Plugins and mods"), msg("With their settings.")],
  ["config", msg("Configuration"), msg("server.properties, whitelist and other settings, without jars and logs.")],
]

/** Chooses what of a server, or of the servers of a job, is backed up. */
export function SelectionField({ value, server, onChange }: { value: Selection; server?: ServerFiles; onChange: (selection: Selection) => void }) {
  return (
    <>
      <FieldSet>
        <FieldLegend variant="label">{t("What to back up")}</FieldLegend>
        <div className="grid gap-3 sm:grid-cols-2">
          {choices.map(([key, label, description]) => {
            const implied = key !== "everything" && value.everything
            return (
              <FieldLabel key={key} htmlFor={`selection-${key}`}>
                <Field orientation="horizontal" data-disabled={implied}>
                  <Checkbox
                    id={`selection-${key}`}
                    checked={value[key] || implied}
                    disabled={implied}
                    onCheckedChange={(on) => onChange({ ...value, [key]: on === true })}
                  />
                  <FieldContent>
                    <FieldTitle>{t(label)}</FieldTitle>
                    <FieldDescription>{t(description)}</FieldDescription>
                  </FieldContent>
                </Field>
              </FieldLabel>
            )
          })}
        </div>
      </FieldSet>
      {!value.everything && <PathsField paths={value.paths} server={server} onChange={(paths) => onChange({ ...value, paths })} />}
    </>
  )
}

// An example in an empty field, which needs no translation.
const examplePath = "plugins/LuckPerms"

/** Further files and folders to back up: picked on a server, or typed for those other servers have. */
function PathsField({ paths, server, onChange }: { paths: string[]; server?: ServerFiles; onChange: (paths: string[]) => void }) {
  const [browsing, setBrowsing] = useState(false)
  const [typed, setTyped] = useState("")
  const error = pathsError(paths)
  const add = (more: string[]) => onChange([...new Set([...paths, ...more])])
  const addTyped = () => {
    if (typed.trim()) add([typed.trim().replace(/^\/+|\/+$/g, "")])
    setTyped("")
  }
  return (
    <Field data-invalid={!!error}>
      <FieldLabel htmlFor="selection-path">{t("More files and folders")}</FieldLabel>
      {paths.length > 0 && (
        <ul aria-label={t("More files and folders")} className="flex flex-wrap gap-1.5">
          {paths.map((p) => (
            <li key={p} className="flex items-center gap-1 rounded-md bg-muted py-0.5 pr-0.5 pl-2 font-mono text-xs">
              {p}
              <button
                type="button"
                aria-label={t("Remove {{name}}", { name: p })}
                onClick={() => onChange(paths.filter((other) => other !== p))}
                className="grid size-5 place-items-center rounded text-muted-foreground hover:bg-background hover:text-foreground"
              >
                <XIcon className="size-3" />
              </button>
            </li>
          ))}
        </ul>
      )}
      <div className="flex flex-wrap gap-2">
        <Button type="button" variant="outline" size="sm" onClick={() => setBrowsing(true)}>
          <FolderOpenIcon />
          {t("Browse…")}
        </Button>
        <Input
          id="selection-path"
          className="h-8 max-w-64 font-mono text-xs"
          placeholder={examplePath}
          aria-invalid={!!error}
          value={typed}
          onChange={(e) => setTyped(e.target.value)}
          onBlur={addTyped}
          onKeyDown={(e) => {
            if (e.key !== "Enter") return
            e.preventDefault()
            addTyped()
          }}
        />
      </div>
      {error ? (
        <FieldError>{error}</FieldError>
      ) : (
        <FieldDescription>{t("Choose them on a server, or type a path. Paths a server doesn't have are skipped.")}</FieldDescription>
      )}
      {browsing && (
        <PickDialog
          title={t("Files and folders to back up")}
          description={!server && t("Browse one of the servers; the others are backed up with the same paths.")}
          server={server}
          action={t("Add")}
          onClose={() => setBrowsing(false)}
          onPick={(_, picked) => add(picked.map((p) => p.path))}
        />
      )}
    </Field>
  )
}

/** Chooses the storage location for backups among those of the nodes; empty means the default one. */
export function LocationField({
  locations,
  value,
  onChange,
}: {
  locations: string[]
  value: string
  onChange: (location: string) => void
}) {
  const options = [...new Set(["default", ...locations, value || "default"])]
  return (
    <Field>
      <FieldLabel htmlFor="backup-location">{t("Storage location")}</FieldLabel>
      <Select value={value || "default"} onValueChange={(v) => onChange(v === "default" ? "" : v)}>
        <SelectTrigger id="backup-location" className="w-full sm:w-64">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((name) => (
            <SelectItem key={name} value={name}>
              {name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <FieldDescription>
        <Trans
          i18nKey="Backups stay on the server's node, out of the server's reach. The node's administrator adds locations, e.g. on another disk, with <command/>."
          components={{ command: <code className="font-mono text-xs">noryx-agent storage add</code> }}
        />
      </FieldDescription>
    </Field>
  )
}
