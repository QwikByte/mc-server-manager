import { ArrowSquareOutIcon, ArrowsCounterClockwiseIcon, DotsThreeIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { useAccess } from "@/features/access/use-access"
import { useOperation } from "@/features/operations/use-operation"
import { useMovePlayers } from "@/features/players/api"
import type { NodeServer } from "@/features/servers/api"
import { type Backend, type Network, useNetworkAction } from "./api"
import { findServer, key, refOf } from "./servers"

/**
 * Restarts a running game server of a network safely, after its players moved to another server, or only sends its
 * players elsewhere. Restarting needs the permission to restart the server and the proxy, sending players the
 * permission to manage the players of the proxy.
 */
export function BackendActions({ network, backend, servers }: { network: Network; backend: Backend; servers?: NodeServer[] }) {
  const { can } = useAccess()
  const action = useNetworkAction(network.id)
  const move = useMovePlayers(network.id)
  const operation = useOperation()
  const [confirm, setConfirm] = useState(false)
  const server = findServer(servers, backend)
  const { proxy } = network
  const restart = can("servers.restart", proxy.nodeId, proxy.serverId) && can("servers.restart", backend.nodeId, backend.serverId)
  // The proxy sends them to another running server of the network.
  const sendable =
    can("players.manage", proxy.nodeId, proxy.serverId) &&
    findServer(servers, proxy)?.state === "running" &&
    network.backends.some((b) => key(b) !== key(backend) && findServer(servers, b)?.state === "running")
  if (server?.state !== "running" || (!restart && !sendable)) return null
  const ref = refOf(server)

  const restartSafely = () =>
    operation.run((onStart) => action.mutateAsync({ action: "rolling-restart", batch: 1, servers: [ref], onStart }), {
      title: t("Restarting {{name}} safely…", { name: backend.name }),
      notify: true,
      done: () => ({ message: t("Restarted {{name}}", { name: backend.name }) }),
    })

  const sendElsewhere = () =>
    move.mutate(ref, {
      onSuccess: ({ players }) =>
        toast.success(
          players > 0
            ? t("Sent {{count}} players of {{name}} elsewhere", {
                count: players,
                name: backend.name,
                defaultValue_one: "Sent {{count}} player of {{name}} elsewhere",
              })
            : t("Nobody plays on {{name}}", { name: backend.name }),
        ),
      onError: (e) => toast.error(e.message),
    })

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            size="icon-sm"
            variant="ghost"
            className="text-muted-foreground"
            aria-label={t("More actions for {{name}}", { name: backend.name })}
            title={t("More actions")}
          >
            <DotsThreeIcon weight="bold" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-56">
          {restart && (
            <DropdownMenuItem disabled={action.isPending} onSelect={() => setConfirm(true)}>
              <ArrowsCounterClockwiseIcon />
              {t("Restart safely…")}
            </DropdownMenuItem>
          )}
          {sendable && (
            <DropdownMenuItem disabled={move.isPending} onSelect={sendElsewhere}>
              <ArrowSquareOutIcon />
              {t("Send players elsewhere")}
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      {confirm && (
        <ConfirmDialog
          open
          onOpenChange={setConfirm}
          title={t("Restart {{name}} safely?", { name: backend.name })}
          description={t(
            "Its players move to another running server of the network first, the one players join first if it can. The proxy keeps running.",
          )}
          action={t("Restart")}
          onConfirm={restartSafely}
        />
      )}
    </>
  )
}
