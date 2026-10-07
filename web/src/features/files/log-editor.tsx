import { HighlightStyle, StreamLanguage, syntaxHighlighting } from "@codemirror/language"
import { EditorView, ViewPlugin } from "@codemirror/view"
import { Tag } from "@lezer/highlight"
import { useMemo } from "react"
import { levelOf } from "@/features/servers/console-format"
import { CodeEditor } from "./code-editor"

// Warnings and errors stand out, like in the console.
const levels = { error: Tag.define(), warn: Tag.define() }
const language = StreamLanguage.define<null>({
  name: "log",
  token(stream) {
    stream.skipToEnd()
    return levelOf(stream.string) ?? null
  },
  tokenTable: levels,
})
const highlight = syntaxHighlighting(
  HighlightStyle.define([
    { tag: levels.error, color: "var(--console-error)" },
    { tag: levels.warn, color: "var(--console-warn)" },
  ]),
)

/** Puts the cursor at pos once the editor opens, and scrolls there with the line at the given edge. */
const startAt = (pos: number, y: "start" | "end") =>
  ViewPlugin.define((view) => {
    const frame = requestAnimationFrame(() => view.dispatch({ selection: { anchor: pos }, effects: EditorView.scrollIntoView(pos, { y }) }))
    return { destroy: () => cancelAnimationFrame(frame) }
  })

/** A log, read only, opened at its end or, after earlier lines were added, where they end. */
export function LogEditor({ text, filename, earlierEnd }: { text: string; filename: string; earlierEnd?: number }) {
  const extensions = useMemo(
    () => [language, highlight, earlierEnd === undefined ? startAt(text.length, "end") : startAt(earlierEnd, "start")],
    [text, earlierEnd],
  )
  return <CodeEditor ref={null} value={text} filename={filename} readOnly extensions={extensions} onChange={() => {}} />
}
