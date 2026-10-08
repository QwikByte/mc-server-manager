import { t } from "i18next"
import { Checkbox } from "@/components/ui/checkbox"
import type { Group } from "./api"

/** Chooses the groups of a user. */
export function GroupPicker({ groups, value, onChange }: { groups: Group[]; value: string[]; onChange: (groups: string[]) => void }) {
  if (groups.length === 0) return <p className="text-sm text-muted-foreground">{t("No groups yet.")}</p>
  return (
    <ul className="grid gap-2">
      {groups.map((g) => (
        <li key={g.id}>
          <label className="flex cursor-pointer items-start gap-3 rounded-lg p-3 ring-1 ring-border hover:bg-muted/50 has-data-[state=checked]:bg-primary/5 has-data-[state=checked]:ring-primary/30">
            <Checkbox
              className="mt-0.5"
              checked={value.includes(g.id)}
              onCheckedChange={(on) => onChange(on === true ? [...value, g.id] : value.filter((id) => id !== g.id))}
            />
            <span className="min-w-0 space-y-0.5">
              <span className="block text-sm font-medium">{g.name}</span>
              {g.description && <span className="block text-xs text-muted-foreground">{g.description}</span>}
            </span>
          </label>
        </li>
      ))}
    </ul>
  )
}
