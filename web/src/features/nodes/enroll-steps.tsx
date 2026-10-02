import { CaretRightIcon } from "@phosphor-icons/react"
import type { ReactNode } from "react"
import { CopyField } from "@/components/copy-field"
import { formatDateTime } from "@/lib/format"
import type { JoinToken } from "./api"

/** Explains how to connect the agent of a node, using a freshly issued join token. */
export function EnrollSteps({ token: { joinToken, joinTokenExpiresAt, installCommand } }: { token: JoinToken }) {
  return (
    <div className="space-y-5">
      <ol className="space-y-5">
        <Step number={1}>
          <p>
            Run this command on the node before {formatDateTime(joinTokenExpiresAt)}. It installs or updates the agent, offers to install
            Docker if it's missing, connects the agent with a token that works only once and starts it.
          </p>
          <CopyField label="Install command" prefix="$" value={installCommand} />
        </Step>
        <Step number={2}>The node appears as online a few seconds later.</Step>
      </ol>
      <details className="group rounded-lg border px-3 py-2 text-sm">
        <summary className="flex cursor-pointer list-none items-center gap-1.5 text-muted-foreground transition-colors hover:text-foreground [&::-webkit-details-marker]:hidden">
          <CaretRightIcon className="size-3.5 transition-transform group-open:rotate-90" />
          Installed the agent another way?
        </summary>
        <div className="mt-3 space-y-2">
          <p>
            Connect it with this command, then start or restart it, e.g. with{" "}
            <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs whitespace-nowrap">sudo systemctl restart mcsm-agent</code>.
          </p>
          <CopyField label="Enroll command" prefix="$" value={`sudo mcsm-agent enroll ${joinToken}`} />
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
