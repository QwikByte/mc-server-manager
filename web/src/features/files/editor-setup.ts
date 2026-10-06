import { HighlightStyle, StreamLanguage, type StreamParser } from "@codemirror/language"
import { EditorState, type Extension } from "@codemirror/state"
import { EditorView } from "@codemirror/view"
import { tags as t } from "@lezer/highlight"
import i18next from "i18next"
import { msg } from "@/lib/i18n"

// Languages are loaded with the first file that needs them.
const json = () => import("@codemirror/lang-json").then((m) => m.json())
const yaml = () => import("@codemirror/lang-yaml").then((m) => m.yaml())
const legacy = (mode: Promise<StreamParser<unknown>>) => mode.then((m) => StreamLanguage.define(m))
const languages: Record<string, () => Promise<Extension> | undefined> = {
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

/** Loads the language of a file by its extension, if the editor knows it. */
export const languageOf = (filename: string) => languages[filename.split(".").pop()?.toLowerCase() ?? ""]?.()

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
export const theme = EditorView.theme(
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

export const highlight = HighlightStyle.define([
  {
    tag: [t.propertyName, t.definition(t.propertyName), t.definition(t.variableName), t.attributeName, t.heading],
    color: "var(--console-command)",
  },
  { tag: [t.string, t.special(t.string), t.quote], color: "var(--code-string)" },
  { tag: [t.number, t.bool, t.null, t.atom], color: "var(--code-literal)" },
  { tag: [t.keyword, t.tagName], color: "var(--code-keyword)" },
  { tag: [t.comment, t.meta], color: "var(--console-muted)", fontStyle: "italic" },
])

/** The texts of CodeMirror in the panel's language. */
export const translated = () => EditorState.phrases.of(Object.fromEntries(phrases.map((p) => [p, i18next.t(p)])))
