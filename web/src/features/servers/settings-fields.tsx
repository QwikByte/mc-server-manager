import { t } from "i18next"
import { Field, FieldContent, FieldDescription, FieldLabel, FieldTitle } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { formatMegabytes } from "@/lib/format"
import { msg } from "@/lib/i18n"
import type { RestartPolicy } from "./api"
import { containerMemoryMb, memoryOptionsMb } from "./server-types"

// Fields shared by the settings of a server and the templates for new servers.

const javaVersions: [value: string, label: string][] = [
  ["", msg("Newest")],
  ["25", "Java 25"],
  ["21", "Java 21"],
  ["17", "Java 17"],
  ["11", "Java 11"],
  ["8", "Java 8"],
]

const restartPolicies: [RestartPolicy, string, string][] = [
  ["always", msg("Always running"), msg("Starts with the node and after a crash, unless you stopped it or it crashed 5 times in a row.")],
  ["on_crash", msg("After a crash"), msg("Starts again when it crashes, up to 5 times in a row.")],
  ["never", msg("Only manually"), msg("Starts only when you start it.")],
]

/** Chooses the memory of a server. freeMb, if the node limits it, leaves out what doesn't fit. */
export function MemoryField({
  id,
  value,
  onChange,
  freeMb,
}: {
  id: string
  value: number
  onChange: (mb: number) => void
  freeMb?: number
}) {
  const fits = (mb: number) => freeMb === undefined || containerMemoryMb(mb) <= freeMb
  return (
    <Field>
      <FieldLabel htmlFor={id}>{t("Memory")}</FieldLabel>
      <Select value={String(value)} onValueChange={(v) => onChange(Number(v))}>
        <SelectTrigger id={id} className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {[...new Set([...memoryOptionsMb, value])]
            .sort((a, b) => a - b)
            .map((mb) => (
              <SelectItem key={mb} value={String(mb)} disabled={!fits(mb)}>
                {formatMegabytes(mb)}
                {!fits(mb) && <span className="text-muted-foreground">{t("More than the node has left")}</span>}
              </SelectItem>
            ))}
        </SelectContent>
      </Select>
      {freeMb !== undefined && (
        <FieldDescription>
          {t("{{free}} left on the node, for the server and what Java needs besides it", { free: formatMegabytes(Math.max(0, freeMb)) })}
        </FieldDescription>
      )}
    </Field>
  )
}

export function RestartPolicyField({ value, onChange }: { value: RestartPolicy; onChange: (policy: RestartPolicy) => void }) {
  return (
    <RadioGroup
      value={value}
      onValueChange={(v) => onChange(v as RestartPolicy)}
      aria-label={t("When the server starts")}
      className="gap-3 sm:grid-cols-3"
    >
      {restartPolicies.map(([policy, label, description]) => (
        <FieldLabel key={policy} htmlFor={`restart-${policy}`}>
          <Field orientation="horizontal" className="items-start">
            <FieldContent>
              <FieldTitle>{t(label)}</FieldTitle>
              <FieldDescription>{t(description)}</FieldDescription>
            </FieldContent>
            <RadioGroupItem id={`restart-${policy}`} value={policy} />
          </Field>
        </FieldLabel>
      ))}
    </RadioGroup>
  )
}

/** Java version and Aikar's flags, which only game servers have. */
export function JavaFields({
  java,
  aikarFlags,
  onChange,
}: {
  java: string
  aikarFlags: boolean
  onChange: (change: { java?: string; aikarFlags?: boolean }) => void
}) {
  return (
    <>
      <Field>
        <FieldLabel htmlFor="settings-java">{t("Java version")}</FieldLabel>
        <Select value={java || "newest"} onValueChange={(v) => onChange({ java: v === "newest" ? "" : v })}>
          <SelectTrigger id="settings-java" className="w-full sm:w-64">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {javaVersions.map(([value, label]) => (
              <SelectItem key={label} value={value || "newest"}>
                {t(label)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <FieldDescription>
          {t(
            "Minecraft 1.20.5 and newer needs Java 21 or newer, 1.18 to 1.20.4 Java 17, and versions up to 1.16 run best on Java 8 or 11.",
          )}
        </FieldDescription>
      </Field>
      <Field orientation="horizontal">
        <Switch id="settings-aikar" checked={aikarFlags} onCheckedChange={(on) => onChange({ aikarFlags: on })} />
        <FieldContent>
          <FieldLabel htmlFor="settings-aikar">{t("Aikar's flags")}</FieldLabel>
          <FieldDescription>{t("Garbage collector tuning recommended for Paper and its forks, which reduces lag spikes.")}</FieldDescription>
        </FieldContent>
      </Field>
    </>
  )
}

/** JVM options, one per line. */
export function JvmOptionsField({ value, onChange }: { value: string; onChange: (lines: string) => void }) {
  return (
    <Field>
      <FieldLabel htmlFor="settings-jvm">{t("JVM options")}</FieldLabel>
      <Textarea
        id="settings-jvm"
        rows={3}
        className="font-mono"
        // i18next-instrument-ignore-next-line: an example of what to enter
        placeholder="-Dfile.encoding=UTF-8"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      <FieldDescription>
        {t("One option per line. The memory is set above, not here. Options that run code, such as Java agents, aren't allowed.")}
      </FieldDescription>
    </Field>
  )
}

export function CpuLimitField({ value, onChange, cpus }: { value: number; onChange: (cores: number) => void; cpus?: number }) {
  return (
    <Field>
      <FieldLabel htmlFor="settings-cpu">{t("CPU limit")}</FieldLabel>
      <Input
        id="settings-cpu"
        type="number"
        min={0}
        max={cpus}
        step={0.5}
        className="w-full font-mono sm:w-40"
        value={value}
        onChange={(e) => onChange(e.target.valueAsNumber || 0)}
      />
      <FieldDescription>
        {cpus
          ? t("CPU cores the server may use of the node's {{count}}. 0 means no limit, which suits most servers.", { count: cpus })
          : t("CPU cores the server may use. 0 means no limit, which suits most servers.")}
      </FieldDescription>
    </Field>
  )
}
