import { CaretRightIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { useId, useState } from "react"
import { Field, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { definitions } from "@/features/properties/schema"
import { cn } from "@/lib/utils"
import { type World, worldDefaults as defaults } from "./world"

const options = (key: string) => {
  const kind = definitions[key].kind
  return kind.type === "select" ? kind.options : []
}

/** Seed, game mode, difficulty and world type of a new game server, folded away behind a summary. */
export function WorldFields({ value, onChange }: { value: World; onChange: (world: World) => void }) {
  const [open, setOpen] = useState(false)
  const id = useId()
  const label = (key: string) => {
    const option = options(key).find(([v]) => v === value[key])
    return option ? t(option[1]) : value[key]
  }
  const seed = value["level-seed"].trim()
  const summary = [
    label("gamemode"),
    label("difficulty"),
    value["level-type"] !== defaults["level-type"] && label("level-type"),
    seed && t("Seed {{seed}}", { seed }),
  ]

  return (
    <div className="grid gap-4">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 rounded-md text-left text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <CaretRightIcon className={cn("size-4 shrink-0 transition-transform motion-reduce:transition-none", open && "rotate-90")} />
        <span className="font-medium">{t("World")}</span>
        <span className="truncate text-muted-foreground">{summary.filter(Boolean).join(" · ")}</span>
      </button>
      {open && (
        <div id={id} className="grid gap-4 sm:grid-cols-2">
          {Object.keys(defaults).map((key) => {
            const field = `world-${key}`
            const set = (v: string) => onChange({ ...value, [key]: v })
            return (
              <Field key={key}>
                <FieldLabel htmlFor={field}>{t(definitions[key].label)}</FieldLabel>
                {key === "level-seed" ? (
                  <Input id={field} className="font-mono" maxLength={256} placeholder={t("Random")} value={value[key]} onChange={(e) => set(e.target.value)} />
                ) : (
                  <Select value={value[key]} onValueChange={(v) => v && set(v)}>
                    <SelectTrigger id={field} className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {/* e.g. a value of a template that isn't among the options */}
                      {value[key] && !options(key).some(([v]) => v === value[key]) && <SelectItem value={value[key]}>{value[key]}</SelectItem>}
                      {options(key).map(([v, l]) => (
                        <SelectItem key={v} value={v}>
                          {t(l)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </Field>
            )
          })}
        </div>
      )}
    </div>
  )
}
