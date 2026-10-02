import { responseError } from "@/lib/api"

/** The target of the master's own commands; other targets are node IDs. */
export const masterTarget = "master"

interface TerminalEvent {
  output?: string
  done?: boolean
  error?: string
}

/**
 * Runs a command line for the master or the agent of a node. Its output is passed to onOutput as it arrives.
 * Resolves with the error message if the command failed; aborting the signal stops the command.
 */
export async function runCommand(
  target: string,
  command: string,
  onOutput: (text: string) => void,
  signal: AbortSignal,
): Promise<string | undefined> {
  const res = await fetch("/api/terminal", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ target, command }),
    signal,
  })
  if (!res.ok || !res.body) throw await responseError(res)
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader()
  let buffer = ""
  for (;;) {
    const { done, value } = await reader.read()
    if (done) throw new Error("The connection to the master was lost.")
    buffer += value
    for (let end = buffer.indexOf("\n"); end >= 0; end = buffer.indexOf("\n")) {
      const event: TerminalEvent = JSON.parse(buffer.slice(0, end))
      buffer = buffer.slice(end + 1)
      if (event.output) onOutput(event.output)
      if (event.done) return event.error
    }
  }
}
