import { CheckIcon, CopyIcon } from "@phosphor-icons/react"
import { useState } from "react"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group"

/** Explains how to connect the agent of a node, using a freshly issued join token. */
export function EnrollSteps({ joinToken }: { joinToken: string }) {
  return (
    <ol className="list-decimal space-y-4 pl-5 text-sm marker:text-muted-foreground">
      <li>Install mcsm-agent on the node.</li>
      <li className="space-y-2">
        <p>Run this command on the node within one hour. The token works only once.</p>
        <CopyCommand command={`mcsm-agent enroll ${joinToken}`} />
      </li>
      <li className="space-y-2">
        <p>Start the agent. The node appears as online a few seconds later.</p>
        <CopyCommand command="mcsm-agent serve" />
      </li>
    </ol>
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
    <InputGroup>
      <InputGroupInput
        readOnly
        value={command}
        aria-label="Command"
        className="font-mono text-xs"
        onFocus={(e) => e.target.select()}
      />
      <InputGroupAddon align="inline-end">
        <InputGroupButton size="icon-xs" aria-label={copied ? "Copied" : "Copy command"} onClick={copy}>
          {copied ? <CheckIcon /> : <CopyIcon />}
        </InputGroupButton>
      </InputGroupAddon>
    </InputGroup>
  )
}
