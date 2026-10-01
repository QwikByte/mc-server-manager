import { indentWithTab } from "@codemirror/commands"
import { json } from "@codemirror/lang-json"
import { yaml } from "@codemirror/lang-yaml"
import { HighlightStyle, StreamLanguage, syntaxHighlighting } from "@codemirror/language"
import { javascript } from "@codemirror/legacy-modes/mode/javascript"
import { properties } from "@codemirror/legacy-modes/mode/properties"
import { shell } from "@codemirror/legacy-modes/mode/shell"
import { toml } from "@codemirror/legacy-modes/mode/toml"
import { xml } from "@codemirror/legacy-modes/mode/xml"
import type { Extension } from "@codemirror/state"
import { EditorView, keymap } from "@codemirror/view"
import { tags as t } from "@lezer/highlight"
import { basicSetup } from "codemirror"
import { type Ref, useEffect, useImperativeHandle, useRef } from "react"

export interface EditorHandle {
  value: () => string
}

const languages: Record<string, () => Extension> = {
  json,
  mcmeta: json,
  yml: yaml,
  yaml,
  properties: () => StreamLanguage.define(properties),
  toml: () => StreamLanguage.define(toml),
  sh: () => StreamLanguage.define(shell),
  js: () => StreamLanguage.define(javascript),
  xml: () => StreamLanguage.define(xml),
}

// Like the console, the editor is dark in both themes.
const theme = EditorView.theme(
  {
    "&": { height: "100%", color: "var(--console-foreground)", backgroundColor: "var(--console)", fontSize: "12px" },
    "&.cm-focused": { outline: "none" },
    ".cm-scroller": { fontFamily: "var(--font-mono)", lineHeight: "1.6" },
    ".cm-content": { caretColor: "var(--console-foreground)", fontVariantLigatures: "none" },
    ".cm-cursor": { borderLeftColor: "var(--console-foreground)" },
    ".cm-gutters": { backgroundColor: "var(--console)", color: "var(--console-muted)", border: "none" },
    ".cm-activeLine, .cm-activeLineGutter": { backgroundColor: "rgb(255 255 255 / 0.04)" },
    "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection": { backgroundColor: "rgb(108 199 172 / 0.28)" },
    ".cm-panels": { backgroundColor: "var(--console)", color: "var(--console-foreground)" },
  },
  { dark: true },
)

const highlight = HighlightStyle.define([
  {
    tag: [t.propertyName, t.definition(t.propertyName), t.definition(t.variableName), t.attributeName, t.heading],
    color: "var(--console-command)",
  },
  { tag: [t.string, t.special(t.string), t.quote], color: "var(--code-string)" },
  { tag: [t.number, t.bool, t.null, t.atom], color: "var(--code-literal)" },
  { tag: [t.keyword, t.tagName], color: "var(--code-keyword)" },
  { tag: [t.comment, t.meta], color: "var(--console-muted)", fontStyle: "italic" },
])

/** CodeMirror editor for a text file. Its content is read through ref. */
export function CodeEditor({
  ref,
  value,
  filename,
  onChange,
}: {
  ref: Ref<EditorHandle>
  value: string
  filename: string
  onChange: () => void
}) {
  const parent = useRef<HTMLDivElement>(null)
  const view = useRef<EditorView>(null)
  const changed = useRef(onChange)
  useEffect(() => {
    changed.current = onChange
  })
  useImperativeHandle(ref, () => ({ value: () => view.current?.state.doc.toString() ?? "" }), [])

  useEffect(() => {
    const extension = filename.split(".").pop()?.toLowerCase() ?? ""
    const editor = new EditorView({
      parent: parent.current!,
      doc: value,
      extensions: [
        basicSetup,
        keymap.of([indentWithTab]),
        languages[extension]?.() ?? [],
        theme,
        syntaxHighlighting(highlight),
        EditorView.updateListener.of((u) => u.docChanged && changed.current()),
        EditorView.contentAttributes.of({ "aria-label": `Contents of ${filename}` }),
      ],
    })
    view.current = editor
    return () => editor.destroy()
  }, [value, filename])

  return (
    <div
      ref={parent}
      className="h-[65vh] min-h-80 overflow-hidden rounded-xl shadow-xl ring-1 shadow-black/10 ring-black/5 focus-within:ring-2 focus-within:ring-ring dark:ring-white/10"
    />
  )
}
