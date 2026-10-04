import { ArrowClockwiseIcon, ArrowsClockwiseIcon, CaretDownIcon, MegaphoneIcon, PlayIcon, PowerIcon, StopIcon, TrashIcon } from "@phosphor-icons/react"
import { useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { FieldError } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import type { Permission } from "@/features/access/permissions"
import { useAccess } from "@/features/access/use-access"
import { type Network, type NetworkAction, useNetworkAction } from "./api"

type Power = "start" | "stop" | "restart"

/** Acts on all servers of a network, and applies or deletes it. */
export function NetworkActions({ network }: { network: Network }) {
  const { can } = useAccess()
  const action = useNetworkAction(network.id)
  const navigate = useNavigate()
  const [dialog, setDialog] = useState<Power | "broadcast">()
  const onAll = (p: Permission) => [network.proxy, ...network.backends].every((s) => can(p, s.nodeId, s.serverId))
  const powers = (["start", "restart", "stop"] as const).filter((p) => onAll(`servers.${p}`))

  function run(a: NetworkAction, loading: string, success: string, then?: () => void) {
    toast.promise(action.mutateAsync(a).then(then), { loading, success, error: (e: Error) => e.message })
  }

  const power: Record<Power, { icon: typeof PlayIcon; label: string; run: () => void; confirm?: string }> = {
    start: {
      icon: PlayIcon,
      label: t("Start all"),
      run: () => run({ action: "start" }, t("Starting {{name}}…", { name: network.name }), t("Started {{name}}", { name: network.name })),
    },
    restart: {
      icon: ArrowClockwiseIcon,
      label: t("Restart all…"),
      confirm: t("The proxy stops first, so that all players leave at once, then the servers restart and the proxy starts last."),
      run: () => run({ action: "restart" }, t("Restarting {{name}}…", { name: network.name }), t("Restarted {{name}}", { name: network.name })),
    },
    stop: {
      icon: StopIcon,
      label: t("Stop all…"),
      confirm: t("The proxy stops first, which disconnects all players, then the servers stop and save their worlds."),
      run: () => run({ action: "stop" }, t("Stopping {{name}}…", { name: network.name }), t("Stopped {{name}}", { name: network.name })),
    },
  }

  return (
    <>
      {powers.length > 0 && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" disabled={action.isPending}>
              <PowerIcon />
              {t("Servers")}
              <CaretDownIcon className="size-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-52">
            {powers.map((p) => {
              const { icon: Icon, label, confirm } = power[p]
              return (
                <DropdownMenuItem key={p} onSelect={() => (confirm ? setDialog(p) : power[p].run())}>
                  <Icon />
                  {label}
                </DropdownMenuItem>
              )
            })}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      {network.backends.every((b) => can("console.commands", b.nodeId, b.serverId)) && (
        <Button variant="outline" onClick={() => setDialog("broadcast")}>
          <MegaphoneIcon />
          {t("Message")}
        </Button>
      )}
      {can("networks.manage") && (
        <>
          <Button
            variant="outline"
            disabled={action.isPending}
            title={t("Configure all servers again, e.g. after a node was offline. Only changed servers restart.")}
            onClick={() => run({ action: "apply" }, t("Applying {{name}}…", { name: network.name }), t("Applied {{name}}", { name: network.name }))}
          >
            <ArrowsClockwiseIcon />
            {t("Apply again")}
          </Button>
          <ConfirmDialog
            trigger={
              <Button variant="outline" className="text-destructive hover:bg-destructive/10 hover:text-destructive">
                <TrashIcon />
                {t("Delete network")}
              </Button>
            }
            title={t("Delete {{name}}?", { name: network.name })}
            description={t("Its servers restart and accept players directly again. The proxy keeps running without forwarding.")}
            action={t("Delete network")}
            destructive
            onConfirm={() =>
              run({ action: "delete" }, t("Deleting {{name}}…", { name: network.name }), t("Deleted {{name}}", { name: network.name }), () =>
                navigate({ to: "/networks" }),
              )
            }
          />
        </>
      )}
      {(dialog === "restart" || dialog === "stop") && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setDialog(undefined)}
          title={dialog === "stop" ? t("Stop {{name}}?", { name: network.name }) : t("Restart {{name}}?", { name: network.name })}
          description={power[dialog].confirm}
          action={dialog === "stop" ? t("Stop all") : t("Restart all")}
          destructive={dialog === "stop"}
          onConfirm={power[dialog].run}
        />
      )}
      {dialog === "broadcast" && (
        <BroadcastDialog
          network={network}
          onOpenChange={(open) => !open && setDialog(undefined)}
          onSend={(message) => action.mutateAsync({ action: "broadcast", message })}
        />
      )}
    </>
  )
}

/** Sends a chat message to the players of all servers of a network. */
function BroadcastDialog({
  network,
  onOpenChange,
  onSend,
}: {
  network: Network
  onOpenChange: (open: boolean) => void
  onSend: (message: string) => Promise<unknown>
}) {
  const [message, setMessage] = useState("")
  const [error, setError] = useState<string>()

  function submit(event: FormEvent) {
    event.preventDefault()
    onSend(message.trim()).then(
      () => {
        toast.success(t("Sent to the players of {{name}}", { name: network.name }))
        onOpenChange(false)
      },
      (e: Error) => setError(e.message),
    )
  }

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{t("Message to {{name}}", { name: network.name })}</DialogTitle>
            <DialogDescription>{t("Every running server of the network shows it in the chat, with say.")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Input
              autoFocus
              value={message}
              maxLength={256}
              aria-label={t("Message")}
              placeholder={t("The network restarts in 5 minutes")}
              onChange={(e) => setMessage(e.target.value)}
            />
            {error && <FieldError>{error}</FieldError>}
          </div>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={!message.trim()}>
              <MegaphoneIcon />
              {t("Send")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
