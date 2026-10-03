import type { ReactNode } from "react"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

const all = "all"

/** A select of a filter whose first option selects everything. */
export function Choice({
  label,
  value,
  onChange,
  everything,
  children,
}: {
  label: string
  value?: string
  onChange: (value: string | undefined) => void
  everything: string
  children: ReactNode
}) {
  return (
    <Select value={value ?? all} onValueChange={(v) => onChange(v === all ? undefined : v)}>
      <SelectTrigger aria-label={label} className="max-sm:flex-1">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={all}>{everything}</SelectItem>
        {children}
      </SelectContent>
    </Select>
  )
}
