import { DesktopIcon, MoonIcon, SunIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { LayoutGroup } from "motion/react"
import { useId } from "react"
import { radios } from "@/components/radios"
import { Highlight } from "@/components/segmented"
import { DropdownMenuRadioGroup, DropdownMenuRadioItem } from "@/components/ui/dropdown-menu"
import { msg } from "@/lib/i18n"
import { setTheme, type Theme, useTheme } from "@/lib/theme"
import { cn } from "@/lib/utils"

const options = [
  { value: "light", label: msg("Light"), icon: SunIcon },
  { value: "dark", label: msg("Dark"), icon: MoonIcon },
  { value: "system", label: msg("System"), icon: DesktopIcon },
] satisfies { value: Theme; label: string; icon: typeof SunIcon }[]

export function ThemeToggle({ className }: { className?: string }) {
  const theme = useTheme()
  const radio = radios(options.map((o) => o.value), theme, setTheme)
  return (
    <div role="radiogroup" aria-label={t("Colour theme")} className={cn("flex rounded-full bg-muted p-0.5", className)}>
      <LayoutGroup id={useId()}>
        {options.map(({ value, label, icon: Icon }) => (
          <button
            key={value}
            {...radio(value)}
            aria-label={t(label)}
            title={t(label)}
            className="relative isolate grid h-7 flex-1 place-items-center rounded-full px-2 text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring aria-checked:text-foreground"
          >
            {theme === value && <Highlight className="rounded-full" />}
            <Icon className="size-3.5" weight={theme === value ? "fill" : "regular"} />
          </button>
        ))}
      </LayoutGroup>
    </div>
  )
}

/** The colour themes as items of a menu. onChoose stores the choice, e.g. for the signed-in user. */
export function ThemeChoices({ onChoose = setTheme }: { onChoose?: (theme: Theme) => void }) {
  const theme = useTheme()
  return (
    <DropdownMenuRadioGroup value={theme} onValueChange={(value) => onChoose(value as Theme)}>
      {options.map(({ value, label, icon: Icon }) => (
        <DropdownMenuRadioItem key={value} value={value}>
          <Icon weight="duotone" />
          {t(label)}
        </DropdownMenuRadioItem>
      ))}
    </DropdownMenuRadioGroup>
  )
}
