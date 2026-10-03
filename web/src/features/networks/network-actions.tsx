import { ArrowsClockwiseIcon, TrashIcon } from "@phosphor-icons/react"
import { useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import type { Network } from "./api"
import { useNetworkChange } from "./network-change"

/** Applies the network to all its servers again, e.g. after a node was offline. */
export function ApplyNetworkButton({ network }: { network: Network }) {
  const { run, isPending } = useNetworkChange(network.id)
  return (
    <Button
      variant="outline"
      disabled={isPending}
      title={t("Configure all servers again, e.g. after a node was offline. Only changed servers restart.")}
      onClick={() =>
        run(
          { action: "apply" },
          { loading: t("Applying {{name}}…", { name: network.name }), success: t("Applied {{name}}", { name: network.name }) },
        )
      }
    >
      <ArrowsClockwiseIcon />
      {t("Apply again")}
    </Button>
  )
}

export function DeleteNetworkButton({ network }: { network: Network }) {
  const { run, isPending } = useNetworkChange(network.id)
  const navigate = useNavigate()
  return (
    <ConfirmDialog
      trigger={
        <Button variant="destructive" disabled={isPending}>
          <TrashIcon />
          {t("Delete network")}
        </Button>
      }
      title={t("Delete {{name}}?", { name: network.name })}
      description={t("Its servers restart and accept players directly again. The proxy keeps running without servers.")}
      action={t("Delete network")}
      destructive
      onConfirm={() =>
        run(
          { action: "delete" },
          { loading: t("Deleting {{name}}…", { name: network.name }), success: t("Deleted {{name}}", { name: network.name }) },
          () => navigate({ to: "/networks" }),
        )
      }
    />
  )
}
