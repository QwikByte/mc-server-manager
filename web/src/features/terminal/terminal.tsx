import { BroomIcon, PaperPlaneRightIcon, StopIcon, TerminalWindowIcon } from "@phosphor-icons/react"
import { type FormEvent, type KeyboardEvent, useEffect, useLayoutEffect, useRef, useState } from "react"
import { runCommand } from "./api"

type EntryState = "running" | "done" | "failed" | "stopped"

/** A command with its output. The prompt tells where it ran. */
interface Entry {
  id: number
  prompt: string
  command: string
  output: string
  state: EntryState
  error?: string
}

const maxEntries = 100
/** Characters of output kept per command, e.g. while following a console. */
const maxOutput = 200_000
const historySteps: Partial<Record<string, number>> = { ArrowUp: -1, ArrowDown: 1 }

/** Runs commands on the master or an agent, one at a time, like a terminal. */
export function Terminal({ target, prompt }: { target: string; prompt: string }) {
  const [entries, setEntries] = useState<Entry[]>([])
  const [input, setInput] = useState("")
  const [busy, setBusy] = useState(false)
  const controller = useRef<AbortController>(undefined)
  const viewport = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const stickToBottom = useRef(true)
  const nextId = useRef(0)
  const history = useRef<string[]>([])
  const historyIndex = useRef(0)

  // Leaving the page stops a command that is still running.
  useEffect(() => () => controller.current?.abort(), [])

  useLayoutEffect(() => {
    const el = viewport.current
    if (el && stickToBottom.current) el.scrollTop = el.scrollHeight
  }, [entries])

  const patch = (id: number, change: (entry: Entry) => Partial<Entry>) =>
    setEntries((list) => list.map((e) => (e.id === id ? { ...e, ...change(e) } : e)))

  async function run(command: string) {
    const id = nextId.current++
    setEntries((list) => [...list, { id, prompt, command, output: "", state: "running" as const }].slice(-maxEntries))
    const abort = new AbortController()
    controller.current = abort
    setBusy(true)
    // Output can arrive in many small parts, so it is rendered once per frame.
    let pending = ""
    let frame = 0
    const flush = () => {
      frame = 0
      const text = pending
      pending = ""
      patch(id, (e) => ({ output: (e.output + text).slice(-maxOutput) }))
    }
    let result: Partial<Entry>
    try {
      const error = await runCommand(
        target,
        command,
        (text) => {
          pending += text
          frame ||= requestAnimationFrame(flush)
        },
        abort.signal,
      )
      result = error ? { state: "failed", error } : { state: "done" }
    } catch (error) {
      result = abort.signal.aborted ? { state: "stopped" } : { state: "failed", error: (error as Error).message }
    }
    cancelAnimationFrame(frame)
    const rest = pending
    patch(id, (e) => ({ ...result, output: (e.output + rest).slice(-maxOutput) }))
    controller.current = undefined
    setBusy(false)
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    const command = input.trim()
    if (!command || busy) return
    history.current = [...history.current.filter((c) => c !== command), command].slice(-50)
    historyIndex.current = history.current.length
    setInput("")
    stickToBottom.current = true
    if (command === "clear") setEntries([])
    else void run(command)
  }

  // Like in a terminal: arrow keys walk through earlier commands, Ctrl+C stops one, Ctrl+L clears.
  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    const el = event.currentTarget
    if (event.ctrlKey && event.key === "c" && busy && el.selectionStart === el.selectionEnd) {
      event.preventDefault()
      controller.current?.abort()
    } else if (event.ctrlKey && event.key === "l") {
      event.preventDefault()
      setEntries([])
    } else if (historySteps[event.key]) {
      event.preventDefault()
      historyIndex.current = Math.min(Math.max(historyIndex.current + historySteps[event.key]!, 0), history.current.length)
      setInput(history.current[historyIndex.current] ?? "")
    }
  }

  return (
    <section
      aria-labelledby="terminal-heading"
      className="overflow-hidden rounded-2xl bg-console text-console-foreground shadow-xl ring-1 shadow-black/10 ring-black/5 [font-variant-ligatures:none] dark:ring-white/10"
    >
      <div className="flex items-center justify-between gap-4 border-b border-white/10 py-2 pr-2 pl-4">
        <h2 id="terminal-heading" className="flex min-w-0 items-center gap-2 text-sm font-semibold">
          <TerminalWindowIcon className="size-4 shrink-0 text-console-command" weight="duotone" />
          <span className="truncate font-mono">{prompt}</span>
        </h2>
        <button
          type="button"
          onClick={() => setEntries([])}
          disabled={entries.length === 0}
          className="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs text-console-muted transition-colors hover:bg-white/10 hover:text-console-foreground disabled:pointer-events-none disabled:opacity-40"
        >
          <BroomIcon className="size-4" />
          Clear
        </button>
      </div>
      <div
        ref={viewport}
        role="log"
        aria-label="Terminal output"
        tabIndex={0}
        onScroll={(e) => {
          const el = e.currentTarget
          stickToBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 32
        }}
        // A click into the output focuses the prompt, unless it selected text to copy.
        onClick={() => window.getSelection()?.isCollapsed && inputRef.current?.focus()}
        className="h-[60vh] min-h-72 space-y-3 overflow-y-auto px-4 py-3 font-mono text-xs leading-5 [scrollbar-color:var(--console-muted)_transparent] [scrollbar-width:thin] focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring"
      >
        {entries.length === 0 ? (
          <p className="text-console-muted">
            Type <span className="text-console-foreground">help</span> to list the commands. ↑ and ↓ repeat earlier commands, Ctrl+C
            stops a running one, and <span className="text-console-foreground">clear</span> empties the terminal.
          </p>
        ) : (
          entries.map((entry) => <EntryView key={entry.id} entry={entry} />)
        )}
      </div>
      <form
        onSubmit={submit}
        className="flex items-center gap-2 border-t border-white/10 bg-white/[0.03] py-1.5 pr-1.5 pl-4 focus-within:bg-white/[0.06]"
      >
        <span aria-hidden className="font-mono text-xs font-bold text-console-command">
          $
        </span>
        <input
          ref={inputRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder={busy ? "Running… Ctrl+C stops the command" : "Type a command, e.g. status"}
          aria-label={`Command for ${prompt}`}
          autoComplete="off"
          autoCapitalize="off"
          spellCheck={false}
          maxLength={1000}
          className="h-9 min-w-0 flex-1 bg-transparent font-mono text-xs outline-none placeholder:text-console-muted"
        />
        {busy ? (
          <button
            type="button"
            aria-label="Stop the command"
            onClick={() => controller.current?.abort()}
            className="grid size-9 place-items-center rounded-lg bg-console-error/15 text-console-error transition-colors hover:bg-console-error/25"
          >
            <StopIcon className="size-4" weight="fill" />
          </button>
        ) : (
          <button
            type="submit"
            aria-label="Run the command"
            disabled={!input.trim()}
            className="grid size-9 place-items-center rounded-lg bg-console-command/15 text-console-command transition-colors hover:bg-console-command/25 disabled:pointer-events-none disabled:opacity-30"
          >
            <PaperPlaneRightIcon className="size-4" weight="fill" />
          </button>
        )}
      </form>
    </section>
  )
}

function EntryView({ entry }: { entry: Entry }) {
  return (
    <div>
      <p className="break-words whitespace-pre-wrap">
        <span className="text-console-command">{entry.prompt}</span>
        <span className="text-console-muted"> $ </span>
        {entry.command}
      </p>
      {/* Not wrapped, so that tables stay aligned on small screens. */}
      {entry.output && <pre className="overflow-x-auto font-mono [scrollbar-width:thin]">{entry.output}</pre>}
      {entry.state === "failed" && <p className="text-console-error">Error: {entry.error}</p>}
      {entry.state === "stopped" && <p className="text-console-warn">^C Stopped.</p>}
      {entry.state === "running" && (
        <p aria-hidden className="animate-pulse text-console-muted">
          ▍
        </p>
      )}
    </div>
  )
}
