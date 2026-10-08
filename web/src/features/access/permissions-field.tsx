import { t } from "i18next"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import type { Area } from "./api"
import type { Permission } from "./permissions"

/**
 * The permissions by area, e.g. of a group or an API token. Choosing one adds the ones it requires; removing one
 * removes those requiring it, so a group never has a permission without what it needs.
 */
export function PermissionsField({
  catalog,
  value,
  onChange,
  scoped,
}: {
  catalog: Area[]
  value: Permission[]
  onChange: (permissions: Permission[]) => void
  scoped: boolean
}) {
  const requires = new Map(catalog.flatMap((a) => a.permissions).map((p) => [p.id, p.requires ?? []]))
  const withRequired = (ids: Permission[]) => {
    const all = new Set(ids)
    for (const id of all) for (const r of requires.get(id) ?? []) all.add(r)
    return [...all]
  }
  const add = (ids: Permission[]) => onChange(withRequired([...value, ...ids]))
  const remove = (ids: Permission[]) => onChange(value.filter((p) => !withRequired([p]).some((r) => ids.includes(r))))

  return (
    <div className="grid gap-6">
      {catalog.map((area) => {
        const ids = area.permissions.map((p) => p.id)
        const all = ids.every((id) => value.includes(id))
        return (
          <fieldset key={area.name} className="grid gap-2">
            <div className="flex items-center justify-between gap-3">
              <legend className="text-sm font-semibold">{area.name}</legend>
              <Button type="button" variant="ghost" size="xs" onClick={() => (all ? remove(ids) : add(ids))}>
                {all ? t("Clear") : t("Choose all")}
              </Button>
            </div>
            <div className="grid gap-2 sm:grid-cols-2">
              {area.permissions.map((p) => (
                <label
                  key={p.id}
                  className="flex cursor-pointer items-start gap-3 rounded-lg p-3 ring-1 ring-border hover:bg-muted/50 has-disabled:cursor-default has-data-[state=checked]:bg-primary/5 has-data-[state=checked]:ring-primary/30"
                >
                  <Checkbox
                    className="mt-0.5"
                    checked={value.includes(p.id)}
                    onCheckedChange={(on) => (on === true ? add([p.id]) : remove([p.id]))}
                  />
                  <span className="min-w-0 space-y-0.5">
                    <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm font-medium">
                      {p.label}
                      {scoped && !p.scoped && <Pill tone="neutral">{t("Everywhere")}</Pill>}
                    </span>
                    {p.description && <span className="block text-xs text-muted-foreground">{p.description}</span>}
                  </span>
                </label>
              ))}
            </div>
          </fieldset>
        )
      })}
    </div>
  )
}
