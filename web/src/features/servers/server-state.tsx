import { WarningCircleIcon, WarningIcon } from "@phosphor-icons/react"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { Trans } from "react-i18next"
import { Callout } from "@/components/callout"
import { StatusBadge, StatusDot } from "@/components/status"
import { toneDots } from "@/components/tone"
import { msg } from "@/lib/i18n"
import { cn } from "@/lib/utils"
import { type Server, type ServerState, usePendingAction } from "./api"
import { serverStates, states } from "./server-types"

const pendingStates = {
  start: { tone: "warning", label: msg("Starting…"), pulse: true },
  stop: { tone: "warning", label: msg("Stopping…"), pulse: true },
  restart: { tone: "warning", label: msg("Restarting…"), pulse: true },
  delete: { tone: "destructive", label: msg("Deleting…"), pulse: true },
} as const

/** The state of a server, or the action this browser runs on it, e.g. stopping. */
export function ServerStateBadge({ server, nodeId }: { server: Server; nodeId: string }) {
  const pending = usePendingAction(nodeId, server.id)
  return <StatusBadge status={pending && pending !== "command" ? pendingStates[pending] : serverStates[server.state]} />
}

/** How many servers are in each state, as a bar of coloured parts and their counts. */
export function StateBar({ servers, className }: { servers: { state: ServerState }[]; className?: string }) {
  const counts = states.map((state) => ({ state, count: servers.filter((s) => s.state === state).length })).filter((c) => c.count > 0)
  const summary = counts.map(({ state, count }) => `${count} ${t(serverStates[state].label)}`).join(", ")
  return (
    <div className={cn("space-y-2", className)}>
      <div role="img" aria-label={summary} className="flex h-1.5 gap-0.5 overflow-hidden rounded-full bg-muted">
        {counts.map(({ state, count }) => (
          <span key={state} className={cn("h-full", toneDots[serverStates[state].tone])} style={{ flexGrow: count }} />
        ))}
      </div>
      <p aria-hidden className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
        {counts.map(({ state, count }) => (
          <span key={state} className="inline-flex items-center gap-1.5">
            <StatusDot status={serverStates[state]} />
            <span className="font-medium text-foreground tabular-nums">{count}</span>
            {t(serverStates[state].label)}
          </span>
        ))}
      </p>
    </div>
  )
}

/** Tells that a server crashes, or stopped because it crashed, and where to find out why. */
export function CrashNotice({ server }: { server: Server }) {
  const { state, crashes, exitCode } = server
  if (crashes === 0 || (state !== "crashing" && state !== "stopped")) return null

  return (
    <Callout
      tone="destructive"
      icon={WarningCircleIcon}
      role="alert"
      className="mb-6"
      title={
        state === "crashing"
          ? t("{{name}} keeps crashing", { name: server.name })
          : t("{{name}} stopped after crashing", { name: server.name })
      }
    >
      <p>
        {[
          exitCode
            ? t("It crashed {{count}} times since it was started, last with exit code {{code}}.", {
                count: crashes,
                code: exitCode,
                defaultValue_one: "It crashed once since it was started, with exit code {{code}}.",
              })
            : t("It crashed {{count}} times since it was started.", {
                count: crashes,
                defaultValue_one: "It crashed once since it was started.",
              }),
          t("The console shows why."),
          state === "crashing"
            ? t("The node starts it again on its own, and stops it if it keeps crashing.")
            : t("Fix the cause, then start it again."),
          exitCode === 137 && t("Exit code 137 means the server was killed, often because it ran out of memory."),
        ]
          .filter(Boolean)
          .join(" ")}
      </p>
    </Callout>
  )
}

/** Warns that a server starts with JVM options set before the agent refused them, until they are removed. */
export function RefusedOptionsNotice({ server, nodeId, canEdit }: { server: Server; nodeId: string; canEdit: boolean }) {
  const refused = server.refusedJvmOptions ?? []
  if (refused.length === 0) return null
  return (
    <Callout
      tone="warning"
      icon={WarningIcon}
      role="note"
      className="mb-6"
      title={t("{{name}} starts with JVM options that are refused now", { name: server.name })}
    >
      <p className="font-mono break-all">{refused.join(" ")}</p>
      <p>
        {t("They can load or run code, so they can't be set anymore. The server keeps them until they are removed.")}{" "}
        {canEdit && (
          <Trans
            i18nKey="<link>Remove them in the settings.</link>"
            components={{
              link: (
                <Link
                  to="/nodes/$nodeId/servers/$serverId/settings"
                  params={{ nodeId, serverId: server.id }}
                  className="font-medium underline-offset-4 hover:underline"
                />
              ),
            }}
          />
        )}
      </p>
    </Callout>
  )
}
