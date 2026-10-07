import { t } from "i18next"
import { Segmented } from "@/components/segmented"
import { Pill } from "@/components/status"
import { Switch } from "@/components/ui/switch"
import type { InstallResult } from "./api"
import { batches, type useRestart } from "./use-restart"

/** Offers to restart the running servers whose plugins change, as most plugins only load when the server starts. */
export function RestartOption({ restart }: { restart: ReturnType<typeof useRestart> }) {
  const { running, on, setOn, batch, setBatch, may } = restart
  if (running.length === 0) return null
  return (
    <div className="grid gap-3 rounded-xl p-4 ring-1 ring-foreground/8">
      <label className="flex items-start gap-3 text-sm">
        <Switch className="mt-0.5" checked={on && may} disabled={!may} onCheckedChange={setOn} />
        <span className="space-y-0.5">
          <span className="block font-medium">
            {t("Restart the {{count}} running servers afterwards", { count: running.length, defaultValue_one: "Restart the running server afterwards" })}
          </span>
          <span className="block text-muted-foreground">
            {may
              ? t("Servers load plugins when they start. The game servers of a network restart a few at a time, so that it stays open; others restart at once.")
              : t("You need the permission to restart all of them for this.")}
          </span>
        </span>
      </label>
      {on && may && (
        <div className="flex flex-wrap items-center gap-3 pl-12 text-sm">
          <span>{t("Servers of a network at a time")}</span>
          <Segmented label={t("Servers of a network at a time")} value={batch} onChange={setBatch} options={batches.map((b) => ({ value: b, label: b }))} />
        </div>
      )}
    </div>
  )
}

/** Whether a server was restarted to load what changed, or still needs a restart. */
export function RestartPill({ result }: { result: InstallResult }) {
  if (result.restarted) return <Pill tone="success">{t("Restarted")}</Pill>
  if (result.restart) return <Pill tone="warning">{t("Needs a restart")}</Pill>
  return null
}
