import { Field, FieldContent, FieldDescription, FieldLabel, FieldTitle } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { formatMegabytes } from "@/lib/format"
import type { RestartPolicy } from "./api"
import { memoryOptionsMb } from "./server-types"

// Fields shared by the settings of a server and the templates for new servers.

const javaVersions: [value: string, label: string][] = [
  ["", "Newest"],
  ["25", "Java 25"],
  ["21", "Java 21"],
  ["17", "Java 17"],
  ["11", "Java 11"],
  ["8", "Java 8"],
]

const restartPolicies: [RestartPolicy, string, string][] = [
  ["always", "Always running", "Starts with the node and after a crash, unless you stopped it."],
  ["on_crash", "After a crash", "Starts again when it crashes."],
  ["never", "Only manually", "Starts only when you start it."],
]

export function MemoryField({ id, value, onChange }: { id: string; value: number; onChange: (mb: number) => void }) {
  return (
    <Field>
      <FieldLabel htmlFor={id}>Memory</FieldLabel>
      <Select value={String(value)} onValueChange={(v) => onChange(Number(v))}>
        <SelectTrigger id={id} className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {[...new Set([...memoryOptionsMb, value])]
            .sort((a, b) => a - b)
            .map((mb) => (
              <SelectItem key={mb} value={String(mb)}>
                {formatMegabytes(mb)}
              </SelectItem>
            ))}
        </SelectContent>
      </Select>
    </Field>
  )
}

export function RestartPolicyField({ value, onChange }: { value: RestartPolicy; onChange: (policy: RestartPolicy) => void }) {
  return (
    <RadioGroup
      value={value}
      onValueChange={(v) => onChange(v as RestartPolicy)}
      aria-label="When the server starts"
      className="gap-3 sm:grid-cols-3"
    >
      {restartPolicies.map(([policy, label, description]) => (
        <FieldLabel key={policy} htmlFor={`restart-${policy}`}>
          <Field orientation="horizontal" className="items-start">
            <FieldContent>
              <FieldTitle>{label}</FieldTitle>
              <FieldDescription>{description}</FieldDescription>
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
        <FieldLabel htmlFor="settings-java">Java version</FieldLabel>
        <Select value={java || "newest"} onValueChange={(v) => onChange({ java: v === "newest" ? "" : v })}>
          <SelectTrigger id="settings-java" className="w-full sm:w-64">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {javaVersions.map(([value, label]) => (
              <SelectItem key={label} value={value || "newest"}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <FieldDescription>
          Minecraft 1.20.5 and newer needs Java 21 or newer, 1.18 to 1.20.4 Java 17, and versions up to 1.16 run best on Java 8 or 11.
        </FieldDescription>
      </Field>
      <Field orientation="horizontal">
        <Switch id="settings-aikar" checked={aikarFlags} onCheckedChange={(on) => onChange({ aikarFlags: on })} />
        <FieldContent>
          <FieldLabel htmlFor="settings-aikar">Aikar's flags</FieldLabel>
          <FieldDescription>Garbage collector tuning recommended for Paper and Purpur, which reduces lag spikes.</FieldDescription>
        </FieldContent>
      </Field>
    </>
  )
}

/** JVM options, one per line. */
export function JvmOptionsField({ value, onChange }: { value: string; onChange: (lines: string) => void }) {
  return (
    <Field>
      <FieldLabel htmlFor="settings-jvm">JVM options</FieldLabel>
      <Textarea
        id="settings-jvm"
        rows={3}
        className="font-mono"
        placeholder="-Dfile.encoding=UTF-8"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      <FieldDescription>One option per line. The memory is set above, not here.</FieldDescription>
    </Field>
  )
}

export function CpuLimitField({ value, onChange, cpus }: { value: number; onChange: (cores: number) => void; cpus?: number }) {
  return (
    <Field>
      <FieldLabel htmlFor="settings-cpu">CPU limit</FieldLabel>
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
        CPU cores the server may use{cpus ? ` of the node's ${cpus}` : ""}. 0 means no limit, which suits most servers.
      </FieldDescription>
    </Field>
  )
}
