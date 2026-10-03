import { WarningCircleIcon } from "@phosphor-icons/react"
import { Callout } from "@/components/callout"
import { StatusBadge } from "@/components/status"
import type { Server, ServerState } from "./api"
import { serverStates } from "./server-types"

export function ServerStateBadge({ state }: { state: ServerState }) {
  return <StatusBadge status={serverStates[state]} />
}

/** Tells that a server crashes, or stopped because it crashed, and where to find out why. */
export function CrashNotice({ server }: { server: Server }) {
  const { state, crashes, exitCode } = server
  if (crashes === 0 || (state !== "crashing" && state !== "stopped")) return null
  const times = crashes === 1 ? "once" : `${crashes} times`
  const code = exitCode ? `, last with exit code ${exitCode}` : ""
  return (
    <Callout
      tone="destructive"
      icon={WarningCircleIcon}
      role="alert"
      className="mb-6"
      title={state === "crashing" ? `${server.name} keeps crashing` : `${server.name} stopped after crashing`}
    >
      <p>
        It crashed {times} since it was started{code}. The console shows why.{" "}
        {state === "crashing"
          ? "The node starts it again on its own, and stops it if it keeps crashing."
          : "Fix the cause, then start it again."}
        {exitCode === 137 && " Exit code 137 means the server was killed, often because it ran out of memory."}
      </p>
    </Callout>
  )
}
