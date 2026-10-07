import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldContent, FieldDescription, FieldError, FieldGroup, FieldLabel, FieldTitle } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import type { ServerRef } from "@/features/networks/api"
import { findServer } from "@/features/networks/servers"
import { OperationStatus } from "@/features/operations/operation-status"
import { retryAction } from "@/features/operations/retry"
import { type Done, guard, useOperation } from "@/features/operations/use-operation"
import { allServersQuery } from "@/features/servers/api"
import { playerActions } from "./actions"
import { type PlayerAction, type PlayerLists, type PlayerResult, playerListsQuery, useChangePlayer } from "./api"

/** Servers a change can go to, e.g. the server of a player, or a whole network. */
export interface Scope {
  label: string
  servers: ServerRef[]
}

const maxReason = 256

/** The reasons of the bans in the lists, the most frequent first, to ban others for them too; only those a ban may have. */
function banReasons(lists: PlayerLists) {
  const counts = new Map<string, number>()
  for (const { reason = "" } of lists.banned) {
    const r = reason.trim()
    if (r && [...r].length <= maxReason && !/\p{Cc}/u.test(r)) counts.set(r, (counts.get(r) ?? 0) + 1)
  }
  return [...counts.keys()].sort((a, b) => counts.get(b)! - counts.get(a)!)
}

/** Kicks, bans, pardons, whitelists or makes operator a player on the servers of a scope. */
export function PlayerActionDialog({
  action,
  name: fixed,
  scopes,
  onClose,
}: {
  action: PlayerAction
  /** The player; without, the dialog asks for one. */
  name?: string
  scopes: Scope[]
  onClose: () => void
}) {
  const info = playerActions[action]
  const global = action === "whitelist_on" || action === "whitelist_off"
  const [name, setName] = useState(fixed ?? "")
  const [reason, setReason] = useState("")
  const [scope, setScope] = useState(0)
  const change = useChangePlayer()
  const operation = useOperation()
  const { data: servers } = useQuery(allServersQuery)
  const { data: reasons = [] } = useQuery({ ...playerListsQuery(), enabled: action === "ban", select: banReasons })
  const player = name.trim()
  const title = info.title(player || "…")
  const nameOf = (ref: ServerRef) => findServer(servers, ref)?.name ?? ref.serverId

  function done(results: PlayerResult[]): Done {
    const failed = results.filter((r) => r.error)
    const pending = results.filter((r) => r.pending).length
    const lines = [
      results.length === 1 && results[0].output
        ? results[0].output
        : t("Done on {{done}} of {{count}} servers.", { done: results.length - failed.length - pending, count: results.length }),
      pending > 0 &&
        t("{{count}} stopped servers catch up when they next start.", {
          count: pending,
          defaultValue_one: "A stopped server catches up when it next starts.",
        }),
      failed.length > 0 && t("Failed on {{servers}}: {{error}}", { servers: failed.map(nameOf).join(", "), error: failed[0].error }),
    ]
    return {
      message: failed.length === results.length ? t("Nothing changed") : info.done(player),
      description: lines.filter(Boolean).join(" "),
      // The dialog is closed by then, so a notification follows the retry.
      action: retryAction(results, (servers) => run(servers, true)),
      warning: failed.length > 0,
    }
  }

  function run(servers: ServerRef[], notify = false) {
    operation.run(
      (onStart) => change.mutateAsync({ action, name: global ? undefined : player, reason: reason.trim() || undefined, servers, onStart }),
      { title, done, then: onClose, notify },
    )
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    run(scopes[scope].servers)
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-lg" {...guard(change.isPending)}>
        {operation.live ? (
          <OperationStatus
            op={operation.live}
            title={title}
            onBackground={() => {
              operation.background(title)
              onClose()
            }}
            onBack={() => {
              operation.reset()
              change.reset()
            }}
          />
        ) : (
          <form onSubmit={submit} className="grid gap-6">
            <DialogHeader>
              <DialogTitle>{global ? info.title("") : title}</DialogTitle>
              <DialogDescription>{describe(action)}</DialogDescription>
            </DialogHeader>
            <FieldGroup>
              {!fixed && !global && (
                <Field>
                  <FieldLabel htmlFor="player-name">{t("Player")}</FieldLabel>
                  <Input
                    id="player-name"
                    autoFocus
                    required
                    maxLength={17}
                    pattern="\.?[A-Za-z0-9_]{1,16}"
                    autoComplete="off"
                    className="font-mono"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                  />
                  <FieldDescription>{t("Players of the Bedrock Edition have a dot in front of their name, e.g. .Steve.")}</FieldDescription>
                </Field>
              )}
              {info.reason && (
                <Field>
                  <FieldLabel htmlFor="player-reason">{t("Reason")}</FieldLabel>
                  <Input
                    id="player-reason"
                    list="player-reasons"
                    autoFocus={!!fixed}
                    maxLength={maxReason}
                    placeholder={t("Optional, the player sees it")}
                    value={reason}
                    onChange={(e) => setReason(e.target.value)}
                  />
                  <datalist id="player-reasons">
                    {reasons.map((r) => (
                      <option key={r} value={r} />
                    ))}
                  </datalist>
                </Field>
              )}
              {scopes.length > 1 ? (
                <RadioGroup value={String(scope)} onValueChange={(v) => setScope(Number(v))} aria-label={t("Servers")} className="gap-2">
                  {scopes.map((s, i) => (
                    <FieldLabel key={s.label} htmlFor={`player-scope-${i}`}>
                      <Field orientation="horizontal">
                        <FieldContent>
                          <FieldTitle>{s.label}</FieldTitle>
                          <FieldDescription>
                            {t("{{count}} servers", { count: s.servers.length, defaultValue_one: "{{count}} server" })}
                          </FieldDescription>
                        </FieldContent>
                        <RadioGroupItem id={`player-scope-${i}`} value={String(i)} />
                      </Field>
                    </FieldLabel>
                  ))}
                </RadioGroup>
              ) : (
                <FieldDescription>
                  {scopes[0].label} · {t("{{count}} servers", { count: scopes[0].servers.length, defaultValue_one: "{{count}} server" })}
                </FieldDescription>
              )}
              {change.error && <FieldError>{change.error.message}</FieldError>}
            </FieldGroup>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="outline" disabled={change.isPending}>
                  {t("Cancel")}
                </Button>
              </DialogClose>
              <Button
                type="submit"
                variant={info.destructive ? "destructive" : "default"}
                disabled={change.isPending || (!global && !player)}
              >
                <info.icon />
                {global ? info.title("") : title}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  )
}

/** What an action does, beyond its name. */
function describe(action: PlayerAction) {
  switch (action) {
    case "kick":
      return t(
        "The player leaves the server. Velocity sends kicked players to another server of their network, BungeeCord disconnects them.",
      )
    case "ban":
      return t("The player can't join the servers anymore. Stopped servers ban the player once they start.")
    case "op":
      return t("Operators may run any command in the game, like the console.")
    case "whitelist_on":
      return t("Only players on the whitelist may join, and online players who aren't on it are disconnected.")
    default:
      return t("Stopped servers apply the change when they next start.")
  }
}
