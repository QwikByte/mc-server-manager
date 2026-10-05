import { t } from "i18next"
import { useState } from "react"
import { Trans } from "react-i18next"
import { Checkbox } from "@/components/ui/checkbox"
import { Field, FieldContent, FieldDescription, FieldError, FieldLabel, FieldLegend, FieldSet, FieldTitle } from "@/components/ui/field"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { msg } from "@/lib/i18n"
import { pathsError, type Selection } from "./api"

const choices: [key: keyof Omit<Selection, "paths">, label: string, description: string][] = [
  ["everything", msg("Everything"), msg("The whole folder of the server, including the server software.")],
  ["worlds", msg("Worlds"), msg("All worlds, also those added later.")],
  ["plugins", msg("Plugins and mods"), msg("With their settings.")],
  ["config", msg("Configuration"), msg("server.properties, whitelist and other settings, without jars and logs.")],
]

/** Chooses what of a server is backed up. */
export function SelectionField({ value, onChange }: { value: Selection; onChange: (selection: Selection) => void }) {
  const [paths, setPaths] = useState(value.paths.join("\n"))
  const error = pathsError(value.paths)
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
      {!value.everything && (
        <Field data-invalid={!!error}>
          <FieldLabel htmlFor="selection-paths">{t("More files and folders")}</FieldLabel>
          <Textarea
            id="selection-paths"
            rows={3}
            className="font-mono"
            // i18next-instrument-ignore-next-line: an example of what to enter
            placeholder={"plugins/LuckPerms\nbanned-players.json"}
            value={paths}
            aria-invalid={!!error}
            onChange={(e) => {
              setPaths(e.target.value)
              onChange({ ...value, paths: e.target.value.split("\n").flatMap((p) => p.trim() || []) })
            }}
          />
          {error ? (
            <FieldError>{error}</FieldError>
          ) : (
            <FieldDescription>{t("One path per line, inside the server's folder. Paths a server doesn't have are skipped.")}</FieldDescription>
          )}
        </Field>
      )}
    </>
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
