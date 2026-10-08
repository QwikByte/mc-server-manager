import { BellRingingIcon, BellSlashIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { AccountRow } from "@/features/auth/account-row"
import { msg } from "@/lib/i18n"
import { desktopSupported, useDesktopNotifications } from "./desktop"

/** Turns notifications of the operating system about new warnings and errors on or off in this browser. */
export function DesktopNotificationsSetting() {
  const desktop = useDesktopNotifications()
  const turnOn = () =>
    desktop.turnOn().then(
      (on) => on && toast.success(t("Turned on desktop notifications in this browser")),
      (e: Error) => toast.error(e.message),
    )
  const description = !desktopSupported
    ? t("This browser can't show notifications of the panel.")
    : desktop.permission === "denied"
      ? t("Your browser blocks notifications of the panel. Allow them in the settings of the site, then turn them on here.")
      : t("New warnings and errors show as notifications of your operating system while the panel is open in a tab in the background. Only in this browser.")
  return (
    <AccountRow
      icon={desktop.on ? BellRingingIcon : BellSlashIcon}
      tone={desktop.on ? "success" : "neutral"}
      title={t("Desktop notifications")}
      status={desktop.on ? { tone: "success", label: msg("On") } : { tone: "neutral", label: msg("Off") }}
      actions={
        desktopSupported &&
        (desktop.on ? (
          <Button variant="outline" onClick={desktop.turnOff}>
            {t("Turn off")}
          </Button>
        ) : (
          <Button variant="outline" disabled={desktop.permission === "denied"} onClick={() => void turnOn()}>
            {t("Turn on")}
          </Button>
        ))
      }
    >
      {description}
    </AccountRow>
  )
}
