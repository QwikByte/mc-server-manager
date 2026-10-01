import { PaperPlaneRightIcon } from "@phosphor-icons/react"
import { type FormEvent, type KeyboardEvent, useEffect, useLayoutEffect, useRef, useState } from "react"
import { cn } from "@/lib/utils"
import { type Server, useSendCommand } from "./api"
import { serverType } from "./server-types"

type LineKind = "log" | "command" | "output" | "error"
type Connection = "connecting" | "live" | "ended" | "lost" | "failed"

interface Line {
  id: number
  kind: LineKind
  text: string
}

const maxLines = 1000
const historySteps: Partial<Record<string, number>> = { ArrowUp: -1, ArrowDown: 1 }

const connectionText: Record<Connection, string> = {
  connecting: "Connecting…",
  live: "Live",
  ended: "Showing the last output",
  lost: "Connection lost, reconnecting…",
  failed: "The console is not available right now.",
}

function lineClass({ kind, text }: Line) {
  if (kind === "command") return "text-console-command"
  if (kind === "error" || /\b(ERROR|SEVERE|FATAL)\b|Exception/.test(text)) return "text-console-error"
  if (/\bWARN(ING)?\b/.test(text)) return "text-console-warn"
  return undefined
}

/** Live console of a server: its output as it happens and a prompt for commands. */
export function Console({ nodeId, server }: { nodeId: string; server: Server }) {
  const [lines, setLines] = useState<Line[]>([])
  const [connection, setConnection] = useState<Connection>("connecting")
  const [input, setInput] = useState("")
  const send = useSendCommand(nodeId, server.id)
  const viewport = useRef<HTMLDivElement>(null)
  const stickToBottom = useRef(true)
  const nextId = useRef(0)
  const pending = useRef<Line[]>([])
  const frame = useRef(0)
  const history = useRef<string[]>([])
  const historyIndex = useRef(0)

  const live = server.state !== "stopped"
  const proxy = serverType(server.type).proxy

  // Lines arrive in bursts (the initial tail has hundreds), so they are rendered once per frame.
  function append(kind: LineKind, ...texts: string[]) {
    for (const text of texts) pending.current.push({ id: nextId.current++, kind, text })
    frame.current ||= requestAnimationFrame(() => {
      frame.current = 0
      const batch = pending.current
      pending.current = []
      setLines((current) => [...current, ...batch].slice(-maxLines))
    })
  }

  // Reconnects when the server starts or stops, so the console always shows the current run.
  useEffect(() => {
    const source = new EventSource(`/api/nodes/${nodeId}/servers/${server.id}/logs`)
    source.onopen = () => {
      pending.current = []
      setLines([])
      setConnection("live")
    }
    source.onmessage = (event) => append("log", event.data)
    source.addEventListener("end", () => {
      source.close()
      setConnection("ended")
    })
    source.onerror = () => setConnection(source.readyState === EventSource.CLOSED ? "failed" : "lost")
    return () => source.close()
  }, [nodeId, server.id, live])

  useLayoutEffect(() => {
    const el = viewport.current
    if (el && stickToBottom.current) el.scrollTop = el.scrollHeight
  }, [lines])

  function submit(event: FormEvent) {
    event.preventDefault()
    const command = input.trim()
    if (!command) return
    history.current = [...history.current.filter((c) => c !== command), command].slice(-50)
    historyIndex.current = history.current.length
    setInput("")
    stickToBottom.current = true
    append("command", `> ${command}`)
    send.mutate(command, {
      onSuccess: ({ output }) => append("output", ...output.split("\n").filter((line) => line.trim())),
      onError: (error) => append("error", error.message),
    })
  }

  // Arrow keys walk through earlier commands, like in a terminal.
  function browseHistory(event: KeyboardEvent<HTMLInputElement>) {
    const step = historySteps[event.key]
    if (!step) return
    event.preventDefault()
    historyIndex.current = Math.min(Math.max(historyIndex.current + step, 0), history.current.length)
    setInput(history.current[historyIndex.current] ?? "")
  }

  const disabledReason = proxy ? "Proxies don't accept console commands yet" : !live ? "Start the server to send commands" : undefined

  return (
    <section aria-labelledby="console-heading">
      <div className="mb-3 flex flex-wrap items-end justify-between gap-x-4 gap-y-1">
        <h2 id="console-heading" className="heading text-xl">
          Console
        </h2>
        <p role="status" className="text-sm text-muted-foreground">
          {connectionText[connection]}
        </p>
      </div>
      {/* Ligatures are off so that output like "<--" shows exactly what the server printed. */}
      <div className="border bg-console text-console-foreground [font-variant-ligatures:none]">
        <div
          ref={viewport}
          role="log"
          aria-label={`Console output of ${server.name}`}
          tabIndex={0}
          onScroll={(e) => {
            const el = e.currentTarget
            stickToBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 32
          }}
          className="h-[60vh] min-h-72 overflow-y-auto p-3 font-mono text-xs leading-5 focus-visible:outline-1 focus-visible:outline-ring"
        >
          {lines.length === 0 ? (
            <p className="text-console-muted">{connection === "connecting" ? "Loading output…" : "No output yet."}</p>
          ) : (
            lines.map((line) => (
              <div key={line.id} className={cn("break-words whitespace-pre-wrap", lineClass(line))}>
                {line.text}
              </div>
            ))
          )}
        </div>
        <form
          onSubmit={submit}
          className="flex items-center gap-2 border-t border-white/10 py-1 pr-1 pl-3 focus-within:ring-1 focus-within:ring-ring"
        >
          <span aria-hidden className="font-mono text-xs text-console-command">
            &gt;
          </span>
          <input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={browseHistory}
            disabled={disabledReason !== undefined}
            placeholder={disabledReason ?? "Type a command, e.g. say Hello"}
            aria-label="Console command"
            autoComplete="off"
            spellCheck={false}
            maxLength={1000}
            className="h-8 min-w-0 flex-1 bg-transparent font-mono text-xs outline-none placeholder:text-console-muted disabled:cursor-not-allowed"
          />
          <button
            type="submit"
            aria-label="Send command"
            disabled={disabledReason !== undefined || !input.trim()}
            className="grid size-8 place-items-center text-console-muted hover:bg-white/10 hover:text-console-foreground disabled:pointer-events-none disabled:opacity-40"
          >
            <PaperPlaneRightIcon className="size-4" />
          </button>
        </form>
      </div>
    </section>
  )
}
