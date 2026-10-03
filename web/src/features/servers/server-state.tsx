import { WarningCircleIcon } from "@phosphor-icons/react"
import { t } from "i18next"
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
