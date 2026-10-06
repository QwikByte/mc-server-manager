import {
  ArrowClockwiseIcon,
  ArrowsLeftRightIcon,
  ArrowsClockwiseIcon,
  ArrowsCounterClockwiseIcon,
  CaretDownIcon,
  DotsThreeIcon,
  MegaphoneIcon,
  PlayIcon,
  PowerIcon,
  StopIcon,
  TrashIcon,
} from "@phosphor-icons/react"
import { useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Segmented } from "@/components/segmented"
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
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { FieldError } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import type { Permission } from "@/features/access/permissions"
import { useAccess } from "@/features/access/use-access"
import { useOperation } from "@/features/operations/use-operation"
import { type Network, type NetworkAction, useNetworkAction } from "./api"
import { SwapProxyDialog } from "./swap-proxy-dialog"

type Power = "start" | "stop" | "restart" | "rolling"

/** Acts on all servers of a network, and applies it; changing its proxy and deleting it are behind a menu. */
export function NetworkActions({ network }: { network: Network }) {
  const { can } = useAccess()
  const action = useNetworkAction(network.id)
  const operation = useOperation()
  const navigate = useNavigate()
  const [dialog, setDialog] = useState<Power | "broadcast" | "swap" | "delete">()
  const close = (open: boolean) => !open && setDialog(undefined)
  const onAll = (p: Permission) => [network.proxy, ...network.backends].every((s) => can(p, s.nodeId, s.serverId))
  const powers = (["start", "restart", "rolling", "stop"] as const).filter((p) => onAll(`servers.${p === "rolling" ? "restart" : p}`))

  function run(a: NetworkAction, loading: string, success: string, then?: () => void) {
    operation.run((onStart) => action.mutateAsync({ ...a, onStart }), { title: loading, notify: true, done: () => ({ message: success }), then })
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
    rolling: {
      icon: ArrowsCounterClockwiseIcon,
      label: t("Restart server by server…"),
      run: () => setDialog("rolling"),
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
            title={[
              t("Configure all servers again, e.g. after a node was offline. Only changed servers restart."),
              network.bedrockPort > 0 && t("Geyser and Floodgate are updated, and the proxy restarts for an update."),
            ]
              .filter(Boolean)
              .join(" ")}
            onClick={() => run({ action: "apply" }, t("Applying {{name}}…", { name: network.name }), t("Applied {{name}}", { name: network.name }))}
          >
            <ArrowsClockwiseIcon />
            {t("Apply again")}
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="icon" aria-label={t("More actions for {{name}}", { name: network.name })} title={t("More actions")}>
                <DotsThreeIcon weight="bold" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-52">
              <DropdownMenuItem disabled={action.isPending} onSelect={() => setDialog("swap")}>
                <ArrowsLeftRightIcon />
                {t("Change proxy…")}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" disabled={action.isPending} onSelect={() => setDialog("delete")}>
                <TrashIcon />
                {t("Delete network…")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </>
      )}
      {dialog === "swap" && <SwapProxyDialog network={network} onClose={() => setDialog(undefined)} />}
      {dialog === "delete" && (
        <ConfirmDialog
          open
          onOpenChange={close}
          title={t("Delete {{name}}?", { name: network.name })}
          description={t("Its servers restart and accept players directly again, in online mode. The proxy keeps running without forwarding.")}
          action={t("Delete network")}
          destructive
          onConfirm={() =>
            run({ action: "delete" }, t("Deleting {{name}}…", { name: network.name }), t("Deleted {{name}}", { name: network.name }), () =>
              navigate({ to: "/networks" }),
            )
          }
        />
      )}
      {(dialog === "restart" || dialog === "stop") && (
        <ConfirmDialog
          open
          onOpenChange={close}
          title={dialog === "stop" ? t("Stop {{name}}?", { name: network.name }) : t("Restart {{name}}?", { name: network.name })}
          description={power[dialog].confirm}
          action={dialog === "stop" ? t("Stop all") : t("Restart all")}
          destructive={dialog === "stop"}
          onConfirm={power[dialog].run}
        />
      )}
      {dialog === "rolling" && (
        <RollingRestartDialog
          network={network}
          onClose={() => setDialog(undefined)}
          onStart={(batch) =>
            run(
              { action: "rolling-restart", batch },
              t("Restarting {{name}} server by server…", { name: network.name }),
              t("Restarted {{name}} server by server", { name: network.name }),
            )
          }
        />
      )}
      {dialog === "broadcast" && (
        <BroadcastDialog
          network={network}
          onOpenChange={close}
          onSend={(message) => action.mutateAsync({ action: "broadcast", message })}
        />
      )}
    </>
  )
}

const batches = ["1", "2", "5", "10"] as const

/** Restarts the running game servers of a network a few at a time, so that it stays open. */
function RollingRestartDialog({ network, onClose, onStart }: { network: Network; onClose: () => void; onStart: (batch: number) => void }) {
  const [batch, setBatch] = useState<(typeof batches)[number]>("1")
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("Restart {{name}} server by server", { name: network.name })}</DialogTitle>
          <DialogDescription>
            {t(
              "The running game servers restart a few at a time, and the network stays open: their players move to another server first, and the next servers restart once these run again. The servers players join first restart last, one at a time. The proxy keeps running.",
            )}
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-wrap items-center gap-3 text-sm">
          <span>{t("Servers at a time")}</span>
          <Segmented
            label={t("Servers at a time")}
            value={batch}
            onChange={setBatch}
            options={batches.map((b) => ({ value: b, label: b }))}
          />
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">{t("Cancel")}</Button>
          </DialogClose>
          <Button
            onClick={() => {
              onStart(Number(batch))
              onClose()
            }}
          >
            <ArrowsCounterClockwiseIcon />
            {t("Restart")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
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
