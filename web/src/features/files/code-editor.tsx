import { indentWithTab } from "@codemirror/commands"
import { HighlightStyle, StreamLanguage, type StreamParser, syntaxHighlighting } from "@codemirror/language"
import { Compartment, EditorState, type Extension } from "@codemirror/state"
import { EditorView, keymap } from "@codemirror/view"
import { tags as t } from "@lezer/highlight"
import { basicSetup } from "codemirror"
import i18next from "i18next"
import { type Ref, useEffect, useImperativeHandle, useRef } from "react"
import { msg } from "@/lib/i18n"

export interface EditorHandle {
  value: () => string
}

// Languages are loaded with the first file that needs them.
const json = () => import("@codemirror/lang-json").then((m) => m.json())
const yaml = () => import("@codemirror/lang-yaml").then((m) => m.yaml())
const legacy = (mode: Promise<StreamParser<unknown>>) => mode.then((m) => StreamLanguage.define(m))
const languages: Record<string, () => Promise<Extension>> = {
  json,
  mcmeta: json,
  yml: yaml,
  yaml,
  properties: () => legacy(import("@codemirror/legacy-modes/mode/properties").then((m) => m.properties)),
  toml: () => legacy(import("@codemirror/legacy-modes/mode/toml").then((m) => m.toml)),
  sh: () => legacy(import("@codemirror/legacy-modes/mode/shell").then((m) => m.shell)),
  js: () => legacy(import("@codemirror/legacy-modes/mode/javascript").then((m) => m.javascript)),
  xml: () => legacy(import("@codemirror/legacy-modes/mode/xml").then((m) => m.xml)),
}

// The texts of CodeMirror, e.g. of its search panel, which it lets translate.
const phrases = [
  msg("Find"),
  msg("Replace"),
  msg("next"),
  msg("previous"),
  msg("all"),
  msg("match case"),
  msg("regexp"),
  msg("by word"),
  msg("replace"),
  msg("replace all"),
  msg("close"),
  msg("current match"),
  msg("on line"),
  msg("replaced $ matches"),
  msg("replaced match on line $"),
  msg("Go to line"),
  msg("go"),
  msg("Folded lines"),
  msg("Unfolded lines"),
  msg("Fold line"),
  msg("Unfold line"),
  msg("folded code"),
  msg("unfold"),
  msg("to"),
  msg("Selection deleted"),
  msg("Control character"),
  msg("Completions"),
  msg("Diagnostics"),
  msg("No diagnostics"),
]

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
  readOnly = false,
  onChange,
}: {
  ref: Ref<EditorHandle>
  value: string
  filename: string
  readOnly?: boolean
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
    const language = new Compartment()
    const editor = new EditorView({
      parent: parent.current!,
      doc: value,
      extensions: [
        basicSetup,
        keymap.of([indentWithTab]),
        language.of([]),
        theme,
        syntaxHighlighting(highlight),
        EditorView.updateListener.of((u) => u.docChanged && changed.current()),
        EditorView.contentAttributes.of({ "aria-label": i18next.t("Contents of {{name}}", { name: filename }) }),
        EditorState.readOnly.of(readOnly),
        EditorState.phrases.of(Object.fromEntries(phrases.map((p) => [p, i18next.t(p)]))),
      ],
    })
    view.current = editor
    let open = true
    void languages[extension]?.().then((l) => open && editor.dispatch({ effects: language.reconfigure(l) }))
    return () => {
      open = false
      editor.destroy()
    }
  }, [value, filename, readOnly])

  return (
    <div
      ref={parent}
      className="h-[65vh] min-h-80 overflow-hidden rounded-xl shadow-xl ring-1 shadow-black/10 ring-black/5 focus-within:ring-2 focus-within:ring-ring dark:ring-white/10"
    />
  )
}
