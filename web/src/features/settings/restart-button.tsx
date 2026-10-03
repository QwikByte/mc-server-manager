import { ArrowClockwiseIcon } from "@phosphor-icons/react"
import { useIsMutating } from "@tanstack/react-query"
import { t } from "i18next"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import { currentPanel, type Master, type PanelTarget, panelURL, restartKey, samePanel, useRestartMaster } from "./api"

/** Restarts the master after asking, and tells when it is back. next is where the panel is then. */
export function RestartButton({ master, next, label = t("Restart master") }: { master: Master; next: PanelTarget; label?: string }) {
  const restart = useRestartMaster()
  const restarting = useIsMutating({ mutationKey: restartKey }) > 0
  return (
    <ConfirmDialog
      trigger={
        // Not a submit button of the settings form it may be in.
        <Button type="button" size="sm" variant="outline" disabled={restarting}>
          <ArrowClockwiseIcon className={restarting ? "animate-spin" : undefined} />
          {restarting ? t("Restarting…") : label}
        </Button>
      }
      title={t("Restart the master?")}
      description={
        samePanel(next, currentPanel(master))
          ? t(
              "The panel is away for a few seconds. Open consoles and logs reconnect, commands in the terminal stop. Minecraft servers and agents keep running.",
            )
          : t(
              "The panel is away for a few seconds and then listens at {{address}}. Open consoles and logs reconnect, commands in the terminal stop. Minecraft servers and agents keep running.",
              { address: panelURL(next) },
            )
      }
      action={t("Restart")}
      onConfirm={() =>
        toast.promise(restart.mutateAsync({ master, next }), {
          loading: t("Restarting the master…"),
          success: t("The master restarted"),
          error: (e: Error) => e.message,
        })
      }
    />
  )
}
