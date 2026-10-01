import { ArrowsClockwiseIcon, TrashIcon } from "@phosphor-icons/react"
import { useNavigate } from "@tanstack/react-router"
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
      title="Configure all servers again, e.g. after a node was offline. Only changed servers restart."
      onClick={() => run({ action: "apply" }, { loading: `Applying ${network.name}…`, success: `Applied ${network.name}` })}
    >
      <ArrowsClockwiseIcon />
      Apply again
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
          Delete network
        </Button>
      }
      title={`Delete ${network.name}?`}
      description="Its servers restart and accept players directly again. The proxy keeps running without servers."
      action="Delete network"
      destructive
      onConfirm={() =>
        run({ action: "delete" }, { loading: `Deleting ${network.name}…`, success: `Deleted ${network.name}` }, () =>
          navigate({ to: "/networks" }),
        )
      }
    />
  )
}
