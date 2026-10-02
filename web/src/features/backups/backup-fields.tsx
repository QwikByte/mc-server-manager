import { useState } from "react"
import { Checkbox } from "@/components/ui/checkbox"
import { Field, FieldContent, FieldDescription, FieldLabel, FieldLegend, FieldSet, FieldTitle } from "@/components/ui/field"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import type { Selection } from "./api"

const choices: [key: keyof Omit<Selection, "paths">, label: string, description: string][] = [
  ["everything", "Everything", "The whole folder of the server, including the server software."],
  ["worlds", "Worlds", "All worlds, also those added later."],
  ["plugins", "Plugins and mods", "With their settings."],
  ["config", "Configuration", "server.properties, whitelist and other settings, without jars and logs."],
]

/** Chooses what of a server is backed up. */
export function SelectionField({ value, onChange }: { value: Selection; onChange: (selection: Selection) => void }) {
  const [paths, setPaths] = useState(value.paths.join("\n"))
  return (
    <>
      <FieldSet>
        <FieldLegend variant="label">What to back up</FieldLegend>
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
                    <FieldTitle>{label}</FieldTitle>
                    <FieldDescription>{description}</FieldDescription>
                  </FieldContent>
                </Field>
              </FieldLabel>
            )
          })}
        </div>
      </FieldSet>
      {!value.everything && (
        <Field>
          <FieldLabel htmlFor="selection-paths">More files and folders</FieldLabel>
          <Textarea
            id="selection-paths"
            rows={3}
            className="font-mono"
            placeholder={"plugins/LuckPerms\nbanned-players.json"}
            value={paths}
            onChange={(e) => {
              setPaths(e.target.value)
              onChange({ ...value, paths: e.target.value.split("\n").flatMap((p) => p.trim() || []) })
            }}
          />
          <FieldDescription>One path per line, inside the server's folder. Paths a server doesn't have are skipped.</FieldDescription>
        </Field>
      )}
    </>
  )
}

/** Chooses the storage location for backups among those of the nodes; empty means the default one. */
export function LocationField({ locations, value, onChange }: { locations: string[]; value: string; onChange: (location: string) => void }) {
  const options = [...new Set(["default", ...locations, value || "default"])]
  return (
    <Field>
      <FieldLabel htmlFor="backup-location">Storage location</FieldLabel>
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
        Backups stay on the server's node, out of the server's reach. The node's administrator adds locations, e.g. on another disk,
        with <code className="font-mono text-xs">mcsm-agent storage add</code>.
      </FieldDescription>
    </Field>
  )
}
