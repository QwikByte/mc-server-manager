import { DesktopIcon, MoonIcon, SunIcon } from "@phosphor-icons/react"
import { setTheme, type Theme, useTheme } from "@/lib/theme"
import { cn } from "@/lib/utils"

const options = [
  { value: "light", label: "Light", icon: SunIcon },
  { value: "dark", label: "Dark", icon: MoonIcon },
  { value: "system", label: "System", icon: DesktopIcon },
] satisfies { value: Theme; label: string; icon: typeof SunIcon }[]

export function ThemeToggle({ className }: { className?: string }) {
  const theme = useTheme()
  return (
    <div role="radiogroup" aria-label="Colour theme" className={cn("flex rounded-full bg-muted p-0.5", className)}>
      {options.map(({ value, label, icon: Icon }) => (
        <button
          key={value}
          type="button"
          role="radio"
          aria-checked={theme === value}
          aria-label={label}
          title={label}
          onClick={() => setTheme(value)}
          className="grid h-7 flex-1 place-items-center rounded-full px-2 text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring aria-checked:bg-card aria-checked:text-foreground aria-checked:shadow-sm"
        >
          <Icon className="size-3.5" weight={theme === value ? "fill" : "regular"} />
        </button>
      ))}
    </div>
  )
}
