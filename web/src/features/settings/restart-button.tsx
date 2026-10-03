import { ArrowClockwiseIcon } from "@phosphor-icons/react"
import { useIsMutating } from "@tanstack/react-query"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import { type Master, restartKey, useRestartMaster } from "./api"

/** Restarts the master after asking, and tells when it is back. next is where the panel listens then. */
export function RestartButton({ master, next, label = "Restart master" }: { master: Master; next: string; label?: string }) {
  const restart = useRestartMaster()
  const restarting = useIsMutating({ mutationKey: restartKey }) > 0
  return (
    <ConfirmDialog
      trigger={
        // Not a submit button of the settings form it may be in.
        <Button type="button" size="sm" variant="outline" disabled={restarting}>
          <ArrowClockwiseIcon className={restarting ? "animate-spin" : undefined} />
          {restarting ? "Restarting…" : label}
        </Button>
      }
      title="Restart the master?"
      description={
        next === master.panelAddr
          ? "The panel is away for a few seconds. Open consoles and logs reconnect, commands in the terminal stop. Minecraft servers and agents keep running."
          : `The panel is away for a few seconds and then listens at ${next}. Open consoles and logs reconnect, commands in the terminal stop. Minecraft servers and agents keep running.`
      }
      action="Restart"
      onConfirm={() =>
        toast.promise(restart.mutateAsync({ master, next }), {
          loading: "Restarting the master…",
          success: "The master restarted",
          error: (e: Error) => e.message,
        })
      }
    />
  )
}
