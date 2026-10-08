import { ArrowClockwiseIcon, PlayIcon, StopIcon, TagIcon, TerminalIcon, XIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import type { Permission } from "@/features/access/permissions"
import { useAccess } from "@/features/access/use-access"
import type { ServerRef } from "@/features/networks/api"
import { key } from "@/features/networks/servers"
import { retryAction } from "@/features/operations/retry"
import { useOperation } from "@/features/operations/use-operation"
import { type BulkAction, type NodeServer, serverKey, useBulkAction } from "./api"
import { PowerDialog } from "./power-dialog"
import { serverType } from "./server-types"
import { TagsDialog } from "./tags"

type Kind = BulkAction["action"]

/** Each action applies to the selected servers in a fitting state on which the user may do it. */
const actions: Record<Kind, { permission: Permission; running: boolean; icon: typeof PlayIcon; label: () => string }> = {
  start: { permission: "servers.start", running: false, icon: PlayIcon, label: () => t("Start") },
  restart: { permission: "servers.restart", running: true, icon: ArrowClockwiseIcon, label: () => t("Restart") },
  stop: { permission: "servers.stop", running: true, icon: StopIcon, label: () => t("Stop") },
  command: { permission: "console.commands", running: true, icon: TerminalIcon, label: () => t("Command…") },
}

/**
 * Acts on the selected servers at once: starts, restarts or stops them, also after warning their players, sends a
 * command, or changes their tags.
 */
export function BulkBar({ selected, onClear }: { selected: NodeServer[]; onClear: () => void }) {
  const { can } = useAccess()
  const bulk = useBulkAction()
  const operation = useOperation()
  const [dialog, setDialog] = useState<Kind | "tags">()
  const close = (open: boolean) => !open && setDialog(undefined)
  const targets = (kind: Kind) =>
    selected.filter((s) => (s.state !== "stopped") === actions[kind].running && can(actions[kind].permission, s.nodeId, s.id))
  const taggable = selected.filter((s) => can("servers.settings", s.nodeId, s.id))

  function run(action: BulkAction, servers = targets(action.action)) {
    const names = new Map(servers.map((s) => [serverKey(s), s.name]))
    const retry = (failed: ServerRef[]) => {
      const keys = new Set(failed.map(key))
      run(action, servers.filter((s) => keys.has(serverKey(s))))
    }
    operation.run((onStart) => bulk.mutateAsync({ ...action, servers, onStart }), {
      title: progress[action.action](servers.length),
      notify: true,
      done: (results) => {
        const failed = results.filter((r) => r.error)
        if (failed.length === 0) return { message: done[action.action](results.length) }
        return {
          message: t("{{failed}} of {{count}} servers failed", { failed: failed.length, count: results.length }),
          description: failed
            .slice(0, 5)
            .map((r) => `${names.get(key(r))}: ${r.error}`)
            .join("; "),
          action: retryAction(results, retry),
          warning: true,
        }
      },
    })
  }

  return (
    <div className="sticky bottom-4 z-20 mt-6 flex flex-wrap items-center gap-2 rounded-xl bg-popover/90 px-3 py-2.5 shadow-2xl ring-1 ring-foreground/10 backdrop-blur-xl">
      <Button variant="ghost" size="icon-sm" aria-label={t("Clear the selection")} title={t("Clear the selection")} onClick={onClear}>
        <XIcon />
      </Button>
      <p className="mr-auto text-sm font-medium tabular-nums">{t("{{count}} selected", { count: selected.length })}</p>
      {(Object.keys(actions) as Kind[]).map((kind) => {
        const { icon: Icon, label } = actions[kind]
        const count = targets(kind).length
        return (
          <Button
            key={kind}
            size="sm"
            variant={kind === "start" ? "default" : "outline"}
            disabled={count === 0 || bulk.isPending}
            onClick={() => (kind === "start" ? run({ action: "start" }) : setDialog(kind))}
          >
            <Icon />
            {label()}
            <span className="tabular-nums opacity-70">{count}</span>
          </Button>
        )
      })}
      <Button size="sm" variant="outline" disabled={taggable.length === 0} onClick={() => setDialog("tags")}>
        <TagIcon />
        {t("Tags…")}
      </Button>
      {(dialog === "restart" || dialog === "stop") && (
        <PowerDialog
          action={dialog}
          title={
            dialog === "stop"
              ? t("Stop {{count}} servers?", { count: targets("stop").length, defaultValue_one: "Stop {{count}} server?" })
              : t("Restart {{count}} servers?", { count: targets("restart").length, defaultValue_one: "Restart {{count}} server?" })
          }
          description={t("Their players are disconnected: {{names}}.", { names: listNames(targets(dialog)) })}
          // Only the players of running game servers can be warned.
          warnable={targets(dialog).some((s) => s.state === "running" && !serverType(s.type).proxy)}
          canMessage={targets(dialog).every((s) => can("console.commands", s.nodeId, s.id))}
          onConfirm={(warning) => run({ action: dialog, warning })}
          onOpenChange={close}
        />
      )}
      {dialog === "command" && (
        <CommandDialog count={targets("command").length} onOpenChange={close} onSend={(command) => run({ action: "command", command })} />
      )}
      {dialog === "tags" && <TagsDialog servers={taggable} onOpenChange={close} />}
    </div>
  )
}

/** The names of servers, the first few of many. */
function listNames(servers: NodeServer[], max = 8) {
  const names = servers.slice(0, max).map((s) => s.name)
  return servers.length > max ? `${names.join(", ")} ${t("and {{count}} more", { count: servers.length - max })}` : names.join(", ")
}

const progress: Record<Kind, (count: number) => string> = {
  start: (count) => t("Starting {{count}} servers…", { count, defaultValue_one: "Starting {{count}} server…" }),
  restart: (count) => t("Restarting {{count}} servers…", { count, defaultValue_one: "Restarting {{count}} server…" }),
  stop: (count) => t("Stopping {{count}} servers…", { count, defaultValue_one: "Stopping {{count}} server…" }),
  command: (count) =>
    t("Sending the command to {{count}} servers…", { count, defaultValue_one: "Sending the command to {{count}} server…" }),
}

const done: Record<Kind, (count: number) => string> = {
  start: (count) => t("Started {{count}} servers", { count, defaultValue_one: "Started {{count}} server" }),
  restart: (count) => t("Restarted {{count}} servers", { count, defaultValue_one: "Restarted {{count}} server" }),
  stop: (count) => t("Stopped {{count}} servers", { count, defaultValue_one: "Stopped {{count}} server" }),
  command: (count) => t("Sent the command to {{count}} servers", { count, defaultValue_one: "Sent the command to {{count}} server" }),
}

/** Asks for a console command that the running selected servers run. */
function CommandDialog({
  count,
  onOpenChange,
  onSend,
}: {
  count: number
  onOpenChange: (open: boolean) => void
  onSend: (command: string) => void
}) {
  const [command, setCommand] = useState("")

  function submit(event: FormEvent) {
    event.preventDefault()
    onSend(command.trim())
    onOpenChange(false)
  }

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{t("Command for {{count}} servers", { count, defaultValue_one: "Command for {{count}} server" })}</DialogTitle>
            <DialogDescription>{t("Each running server of the selection runs it in its console, e.g. save-all.")}</DialogDescription>
          </DialogHeader>
          <div className="flex items-center rounded-md font-mono ring-1 ring-input focus-within:ring-2 focus-within:ring-ring">
            <span className="pl-3 text-sm text-muted-foreground">/</span>
            <Input
              autoFocus
              value={command}
              maxLength={256}
              aria-label={t("Command")}
              placeholder={t("e.g. save-all")}
              className="border-0 shadow-none ring-0 focus-visible:ring-0"
              onChange={(e) => setCommand(e.target.value)}
            />
          </div>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={!command.trim()}>
              <TerminalIcon />
              {t("Send")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
