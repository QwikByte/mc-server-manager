import type { ReactNode } from "react"
import { CopyField } from "@/components/copy-field"
import { formatDateTime } from "@/lib/format"
import type { JoinToken } from "./api"

/** Explains how to connect the agent of a node, using a freshly issued join token. */
export function EnrollSteps({ token: { joinToken, joinTokenExpiresAt } }: { token: JoinToken }) {
  return (
    <ol className="space-y-5">
      <Step number={1}>Install mcsm-agent on the node.</Step>
      <Step number={2}>
        <p>Run this command on the node before {formatDateTime(joinTokenExpiresAt)}. The token works only once.</p>
        <CopyField label="Command" prefix="$" value={`mcsm-agent enroll ${joinToken}`} />
      </Step>
      <Step number={3}>
        <p>Start the agent. The node appears as online a few seconds later.</p>
        <CopyField label="Command" prefix="$" value="mcsm-agent serve" />
      </Step>
    </ol>
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
