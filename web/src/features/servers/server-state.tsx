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
import { serverStates, serverType, states, statusOf } from "./server-types"

const pendingStates = {
  start: { tone: "warning", label: msg("Starting…"), pulse: true },
  stop: { tone: "warning", label: msg("Stopping…"), pulse: true },
  restart: { tone: "warning", label: msg("Restarting…"), pulse: true },
  delete: { tone: "destructive", label: msg("Deleting…"), pulse: true },
} as const

/** The state of a server, or the action this browser runs on it, e.g. stopping. */
export function ServerStateBadge({ server, nodeId }: { server: Server; nodeId: string }) {
  const pending = usePendingAction(nodeId, server.id)
  return <StatusBadge status={pending && pending !== "command" ? pendingStates[pending] : statusOf(server)} />
}

/** How many servers are in each state, as a bar of coloured parts and their counts. */
export function StateBar({ servers, className }: { servers: { state: ServerState }[]; className?: string }) {
  const counts = states.map((state) => ({ state, count: servers.filter((s) => s.state === state).length })).filter((c) => c.count > 0)
  const summary = counts.map(({ state, count }) => `${count} ${t(serverStates[state].label)}`).join(", ")
  return (
    <div className={cn("space-y-2", className)}>
      <div role="img" aria-label={summary} className="flex h-1.5 gap-0.5 overflow-hidden rounded-[2px] bg-muted">
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

/**
 * Tells that a server crashes, stopped because it crashed, or runs but fails its health check,
 * and where to find out why.
 */
export function CrashNotice({ server, nodeId, canReadFiles }: { server: Server; nodeId: string; canReadFiles: boolean }) {
  const { state, crashes, exitCode } = server
  // Game servers write a report of each crash of Minecraft itself.
  const reports = canReadFiles && !serverType(server.type).proxy && (
    <Trans
      i18nKey="Its <link>crash reports</link> may tell more."
      components={{
        link: (
          <Link
            to="/nodes/$nodeId/servers/$serverId/files"
            params={{ nodeId, serverId: server.id }}
            search={{ path: "crash-reports" }}
            className="font-medium underline underline-offset-4"
          />
        ),
      }}
    />
  )
  if (state === "running" && server.unhealthy) {
    return (
      <Callout tone="warning" icon={WarningIcon} role="alert" className="mb-6" title={t("{{name}} is unhealthy", { name: server.name })}>
        <p>
          {t("It runs, but its health check fails, e.g. as it hangs or doesn't answer players. The console shows what it does; if it doesn't recover, restart it.")}{" "}
          {reports}
        </p>
      </Callout>
    )
  }
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
            ? t("It has crashed {{count}} times since it was started, most recently with exit code {{code}}.", {
                count: crashes,
                code: exitCode,
                defaultValue_one: "It has crashed once since it was started, with exit code {{code}}.",
              })
            : t("It has crashed {{count}} times since it was started.", {
                count: crashes,
                defaultValue_one: "It has crashed once since it was started.",
              }),
          t("The console shows why."),
          state === "crashing"
            ? t("The node starts it again on its own, and stops it if it keeps crashing.")
            : t("Fix the cause, then start it again."),
          exitCode === 137 && t("Exit code 137 means the server was killed, often because it ran out of memory."),
        ]
          .filter(Boolean)
          .join(" ")}{" "}
        {reports}
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
      title={t("{{name}} starts with JVM options that are no longer allowed", { name: server.name })}
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
