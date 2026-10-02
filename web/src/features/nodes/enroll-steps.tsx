import { CheckIcon, CopyIcon } from "@phosphor-icons/react"
import { type ReactNode, useState } from "react"
import { formatDateTime } from "@/lib/format"
import type { JoinToken } from "./api"

/** Explains how to connect the agent of a node, using a freshly issued join token. */
export function EnrollSteps({ token: { joinToken, joinTokenExpiresAt } }: { token: JoinToken }) {
  return (
    <ol className="space-y-5">
      <Step number={1}>Install mcsm-agent on the node.</Step>
      <Step number={2}>
        <p>Run this command on the node before {formatDateTime(joinTokenExpiresAt)}. The token works only once.</p>
        <CopyCommand command={`mcsm-agent enroll ${joinToken}`} />
      </Step>
      <Step number={3}>
        <p>Start the agent. The node appears as online a few seconds later.</p>
        <CopyCommand command="mcsm-agent serve" />
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

function CopyCommand({ command }: { command: string }) {
  const [copied, setCopied] = useState(false)

  async function copy() {
    await navigator.clipboard.writeText(command)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="flex items-center gap-2 rounded-lg bg-console py-1 pr-1 pl-3 text-console-foreground focus-within:ring-2 focus-within:ring-ring">
      <span aria-hidden className="font-mono text-xs text-console-command">
        $
      </span>
      <input
        readOnly
        value={command}
        aria-label="Command"
        onFocus={(e) => e.target.select()}
        className="h-8 min-w-0 flex-1 bg-transparent font-mono text-xs outline-none"
      />
      <button
        type="button"
        aria-label={copied ? "Copied" : "Copy command"}
        onClick={copy}
        className="grid size-7 shrink-0 place-items-center rounded-md text-console-muted transition-colors hover:bg-white/10 hover:text-console-foreground"
      >
        {copied ? <CheckIcon className="size-4 text-console-command" /> : <CopyIcon className="size-4" />}
      </button>
    </div>
  )
}
