import { Field, FieldContent, FieldDescription, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { formatMegabytes } from "@/lib/format"
import type { LimitsForm } from "./limits"

/** Port range and memory limit of a node; memoryMb is the node's memory, if known. */
export function LimitsFields({
  id,
  form,
  onChange,
  memoryMb,
}: {
  id: string
  form: LimitsForm
  onChange: (change: Partial<LimitsForm>) => void
  memoryMb?: number
}) {
  const reserve = Number(form.memoryReserveMb) || 0
  return (
    <>
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
            onChange={(e) => onChange({ portMin: e.target.value })}
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
            onChange={(e) => onChange({ portMax: e.target.value })}
          />
        </div>
        <FieldDescription>New servers get the first free port of the range. Leave both empty to allow any port.</FieldDescription>
      </FieldSet>
      <Field orientation="horizontal">
        <Switch id={`${id}-limit`} checked={form.limitMemory} onCheckedChange={(limitMemory) => onChange({ limitMemory })} />
        <FieldContent>
          <FieldLabel htmlFor={`${id}-limit`}>Limit memory</FieldLabel>
          <FieldDescription>Servers together can't get more memory than the node has, minus a reserve for the system.</FieldDescription>
        </FieldContent>
      </Field>
      {form.limitMemory && (
        <Field>
          <FieldLabel htmlFor={`${id}-reserve`}>Reserve in MB</FieldLabel>
          <Input
            id={`${id}-reserve`}
            type="number"
            min={0}
            className="font-mono sm:w-40"
            value={form.memoryReserveMb}
            onChange={(e) => onChange({ memoryReserveMb: e.target.value })}
          />
          <FieldDescription>
            {memoryMb
              ? `Servers can get up to ${formatMegabytes(Math.max(0, memoryMb - reserve))} of the node's ${formatMegabytes(memoryMb)}. `
              : ""}
            Java uses about a quarter more than the memory a server gets, so keep some room.
          </FieldDescription>
        </Field>
      )}
    </>
  )
}
