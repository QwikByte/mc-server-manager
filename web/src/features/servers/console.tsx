import { PaperPlaneRightIcon, TerminalIcon } from "@phosphor-icons/react"
import { type FormEvent, type KeyboardEvent, useEffect, useLayoutEffect, useRef, useState } from "react"
import { useAccess } from "@/features/access/use-access"
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

const connections: Record<Connection, { text: string; dot: string }> = {
  connecting: { text: "Connecting…", dot: "bg-console-warn animate-pulse" },
  live: { text: "Live", dot: "bg-console-command shadow-[0_0_8px_var(--console-command)]" },
  ended: { text: "Showing the last output", dot: "bg-console-muted" },
  lost: { text: "Connection lost, reconnecting…", dot: "bg-console-warn animate-pulse" },
  failed: { text: "The console is not available right now.", dot: "bg-console-error" },
}

function lineClass({ kind, text }: Line) {
  if (kind === "command") return "text-console-command"
  if (kind === "error" || /\b(ERROR|SEVERE|FATAL)\b|Exception/.test(text)) return "text-console-error"
  if (/\bWARN(ING)?\b/.test(text)) return "text-console-warn"
  return undefined
}

/** Live console of a server: its output as it happens and a prompt for commands. */
export function Console({ nodeId, server }: { nodeId: string; server: Server }) {
  const { can } = useAccess()
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
  // The ID of the latest line, so that connecting again continues after it.
  const lastId = useRef("")

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

  // Connects again when the state of the server changes, e.g. when it starts again after a
  // crash, and when the master renews the stream to check the access again. The console
  // continues after the latest line; without one, it shows the last lines afresh.
  useEffect(() => {
    let source: EventSource
    const connect = () => {
      source = new EventSource(`/api/nodes/${nodeId}/servers/${server.id}/logs?after=${lastId.current}`)
      source.onopen = () => {
        if (!lastId.current) {
          pending.current = []
          setLines([])
        }
        setConnection("live")
      }
      source.onmessage = (event) => {
        lastId.current = event.lastEventId
        append("log", event.data)
      }
      source.addEventListener("end", () => {
        source.close()
        setConnection("ended")
      })
      source.addEventListener("renew", () => {
        source.close()
        connect()
      })
      source.onerror = () => setConnection(source.readyState === EventSource.CLOSED ? "failed" : "lost")
    }
    connect()
    return () => source.close()
  }, [nodeId, server.id, server.state])

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

  const disabledReason = !can("console.commands", nodeId, server.id)
    ? "Your groups don't let you send commands"
    : proxy
      ? "Proxies don't accept console commands yet"
      : !live
        ? "Start the server to send commands"
        : undefined

  return (
    <section
      aria-labelledby="console-heading"
      className="overflow-hidden rounded-2xl bg-console text-console-foreground shadow-xl ring-1 shadow-black/10 ring-black/5 [font-variant-ligatures:none] dark:ring-white/10"
    >
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 border-b border-white/10 px-4 py-3">
        <h2 id="console-heading" className="flex items-center gap-2 text-sm font-semibold">
          <TerminalIcon className="size-4 text-console-command" weight="duotone" />
          Console
        </h2>
        <p role="status" className="flex items-center gap-2 text-xs text-console-muted">
          <span aria-hidden className={cn("size-2 rounded-full", connections[connection].dot)} />
          {connections[connection].text}
        </p>
      </div>
      {/* Ligatures are off so that output like "<--" shows exactly what the server printed. */}
      <div
        ref={viewport}
        role="log"
        aria-label={`Console output of ${server.name}`}
        tabIndex={0}
        onScroll={(e) => {
          const el = e.currentTarget
          stickToBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 32
        }}
        className="h-[60vh] min-h-72 overflow-y-auto px-4 py-3 font-mono text-xs leading-5 [scrollbar-color:var(--console-muted)_transparent] [scrollbar-width:thin] focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring"
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
        className="flex items-center gap-2 border-t border-white/10 bg-white/[0.03] py-1.5 pr-1.5 pl-4 focus-within:bg-white/[0.06]"
      >
        <span aria-hidden className="font-mono text-xs font-bold text-console-command">
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
          className="h-9 min-w-0 flex-1 bg-transparent font-mono text-xs outline-none placeholder:text-console-muted disabled:cursor-not-allowed"
        />
        <button
          type="submit"
          aria-label="Send command"
          disabled={disabledReason !== undefined || !input.trim()}
          className="grid size-9 place-items-center rounded-lg bg-console-command/15 text-console-command transition-colors hover:bg-console-command/25 disabled:pointer-events-none disabled:opacity-30"
        >
          <PaperPlaneRightIcon className="size-4" weight="fill" />
        </button>
      </form>
    </section>
  )
}
