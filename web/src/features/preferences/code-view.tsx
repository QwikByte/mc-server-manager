import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import type { ReactElement } from "react"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { codeSizes, type Settings, useSettings, wraps } from "./api"

/**
 * Chooses how the console, the terminal or the editor shows its text, for all of them where the choice is shared, e.g.
 * the size of the text. The account page has these and more.
 */
export function CodeViewMenu({ trigger, wrap, times }: { trigger: ReactElement; wrap: keyof typeof wraps; times?: boolean }) {
  const { settings, change } = useSettings()
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>{trigger}</DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-60">
        <DropdownMenuLabel>{t("Text size")}</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={settings.codeSize ?? "small"} onValueChange={(v) => change({ codeSize: v as Settings["codeSize"] })}>
          {codeSizes.map((s) => (
            <DropdownMenuRadioItem key={s.value} value={s.value}>
              {t(s.label)}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
        <DropdownMenuSeparator />
        <DropdownMenuCheckboxItem checked={wraps[wrap].on(settings)} onCheckedChange={(on) => change(wraps[wrap].set(on))}>
          {t("Wrap long lines")}
        </DropdownMenuCheckboxItem>
        {times && (
          <DropdownMenuCheckboxItem checked={settings.consoleTimes === "show"} onCheckedChange={(on) => change({ consoleTimes: on ? "show" : "hide" })}>
            {t("Show when each line was written")}
          </DropdownMenuCheckboxItem>
        )}
        <DropdownMenuCheckboxItem checked={settings.codeTheme === "panel"} onCheckedChange={(on) => change({ codeTheme: on ? "panel" : "dark" })}>
          {t("Light in the light theme")}
        </DropdownMenuCheckboxItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/account" hash="code">
            {t("More settings")}
          </Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
