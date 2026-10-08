import { CaretRightIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { type ReactNode, useState } from "react"
import { Trans } from "react-i18next"
import { CopyField } from "@/components/copy-field"
import { Segmented } from "@/components/segmented"
import { formatDateTime } from "@/lib/format"
import type { JoinToken, Runtime } from "./api"
import { runtimeTitle } from "./runtime"

/**
 * Explains how to connect the agent of a node, using a freshly issued join token. The runtime of
 * its servers is chosen for the install command, Docker unless the node runs Podman already.
 */
export function EnrollSteps({
  token: { joinToken, joinTokenExpiresAt, installCommand },
  runtime: initial = "docker",
}: {
  token: JoinToken
  runtime?: Runtime
}) {
  const [runtime, setRuntime] = useState<Runtime>(initial)
  const time = formatDateTime(joinTokenExpiresAt)
  return (
    <div className="space-y-5">
      <ol className="space-y-5">
        <Step number={1}>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="font-medium">{t("Runtime of the servers")}</span>
            <Segmented
              label={t("Runtime of the servers")}
              value={runtime}
              options={(["docker", "podman"] as const).map((value) => ({ value, label: runtimeTitle(value) }))}
              onChange={setRuntime}
            />
          </div>
          <p className="text-muted-foreground">
            {runtime === "podman"
              ? t("Podman runs the servers as root, like Docker. It needs version 4.9 or newer, e.g. of Debian 13, Ubuntu 24.04 or RHEL 9.")
              : t("Docker runs the servers in containers. Choose Podman for nodes that run it instead.")}
          </p>
        </Step>
        <Step number={2}>
          <p>
            {runtime === "podman"
              ? t(
                  "Run this command on the node before {{time}}. It installs or updates the agent, installs Podman if it's missing, connects the agent with a token that works only once and starts it.",
                  { time },
                )
              : t(
                  "Run this command on the node before {{time}}. It installs or updates the agent, offers to install Docker if it's missing, connects the agent with a token that works only once and starts it.",
                  { time },
                )}
          </p>
          <CopyField
            label={t("Install command")}
            prefix="$"
            value={runtime === "podman" ? `${installCommand} --runtime podman` : installCommand}
          />
        </Step>
        <Step number={3}>{t("The node appears as online a few seconds later.")}</Step>
      </ol>
      <details className="group rounded-lg border px-3 py-2 text-sm">
        <summary className="flex cursor-pointer list-none items-center gap-1.5 text-muted-foreground transition-colors hover:text-foreground [&::-webkit-details-marker]:hidden">
          <CaretRightIcon className="size-3.5 transition-transform group-open:rotate-90" />
          {t("Installed the agent another way?")}
        </summary>
        <div className="mt-3 space-y-2">
          <p>
            <Trans
              i18nKey="Connect it with this command, then start or restart it, e.g. with <command/>."
              components={{
                command: (
                  <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs whitespace-nowrap">
                    sudo systemctl restart noryx-agent
                  </code>
                ),
              }}
            />
          </p>
          <CopyField label={t("Enroll command")} prefix="$" value={`sudo noryx-agent enroll ${joinToken}`} />
        </div>
      </details>
    </div>
  )
}

function Step({ number, children }: { number: number; children: ReactNode }) {
  return (
    <li className="flex gap-3">
      <span aria-hidden className="grid size-6 shrink-0 place-items-center rounded-full bg-primary/10 text-xs font-bold text-primary">
        {number}
      </span>
      <div className="min-w-0 flex-1 space-y-2 pt-0.5 text-sm">{children}</div>
    </li>
  )
}
