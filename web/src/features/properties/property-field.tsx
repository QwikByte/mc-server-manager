import { LockSimpleIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { Field, FieldContent, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { MotdField } from "./motd"
import { definition } from "./schema"

/** A property with a control that fits its type. */
export function PropertyField({
  name,
  value,
  locked,
  error,
  changed,
  onChange,
}: {
  name: string
  value: string
  locked?: string
  error?: string
  changed: boolean
  onChange: (value: string) => void
}) {
  const { label, description, kind } = definition(name, value)
  const id = `property-${name}`
  const title = (
    <FieldLabel htmlFor={id} className="flex-wrap gap-x-2">
      {t(label)}
      {label !== name && <span className="font-mono text-xs font-normal text-muted-foreground">{name}</span>}
      {changed && <span className="size-2 rounded-full bg-warning" title={t("Changed")} aria-label={t("Changed")} />}
      {locked && <LockSimpleIcon className="size-3.5 text-muted-foreground" aria-label={t("Managed by the panel")} />}
    </FieldLabel>
  )
  const hint = (locked || description) && <FieldDescription>{locked ?? t(description)}</FieldDescription>

  if (kind.type === "boolean")
    return (
      <Field orientation="horizontal" data-disabled={!!locked}>
        <Switch id={id} checked={value === "true"} disabled={!!locked} onCheckedChange={(on) => onChange(String(on))} />
        <FieldContent>
          {title}
          {hint}
        </FieldContent>
      </Field>
    )

  const unknownOption = kind.type === "select" && !kind.options.some(([v]) => v === value)
  return (
    <Field data-invalid={!!error} data-disabled={!!locked} className={kind.type === "motd" ? "md:col-span-2" : undefined}>
      {title}
      {kind.type === "select" ? (
        <Select value={value} onValueChange={onChange} disabled={!!locked}>
          <SelectTrigger id={id} className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {unknownOption && <SelectItem value={value}>{value}</SelectItem>}
            {kind.options.map(([v, l]) => (
              <SelectItem key={v} value={v}>
                {t(l)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : kind.type === "motd" ? (
        <MotdField id={id} value={value} onChange={onChange} disabled={!!locked} />
      ) : (
        <Input
          id={id}
          type={kind.type === "number" ? "number" : "text"}
          min={kind.type === "number" ? kind.min : undefined}
          max={kind.type === "number" ? kind.max : undefined}
          className={kind.type === "text" && !kind.mono ? undefined : "font-mono"}
          value={value}
          disabled={!!locked}
          aria-invalid={!!error}
          onChange={(e) => onChange(e.target.value)}
        />
      )}
      {error ? <FieldError>{error}</FieldError> : hint}
    </Field>
  )
}
