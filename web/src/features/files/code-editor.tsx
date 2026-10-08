import { indentWithTab } from "@codemirror/commands"
import { indentUnit, syntaxHighlighting } from "@codemirror/language"
import { Compartment, EditorState, type Extension } from "@codemirror/state"
import { EditorView, keymap } from "@codemirror/view"
import { basicSetup } from "codemirror"
import i18next from "i18next"
import { type Ref, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react"
import { useCodeDark, useSettings } from "@/features/preferences/api"
import { checkOf, darkness, highlight, languageOf, theme, translated } from "./editor-setup"

/** A syntax error in the text, at a position of it. */
export interface Problem {
  message: string
  at: number
  line: number
}

export interface EditorHandle {
  value: () => string
  /** Replaces the selection with text and focuses the editor. */
  insert: (text: string) => void
  /** The first syntax error of the text, if the editor can check the file's syntax. */
  problem: () => Promise<Problem | undefined>
  /** Moves the cursor to a position and focuses the editor. */
  reveal: (at: number) => void
}

const none: Extension = []

// Vim's keys load with the first editor that uses them.
const vim = () => import("@replit/codemirror-vim").then((m) => m.vim())

/** How the editor indents: with 2 or 4 spaces or tabs, YAML always with spaces, or else as CodeMirror does. */
function indentation(indent: string | undefined, filename: string): Extension {
  if (indent === "tab" && !/\.ya?ml$/i.test(filename)) return [indentUnit.of("\t"), EditorState.tabSize.of(4)]
  const spaces = indent === "4" ? 4 : indent ? 2 : 0
  return spaces ? [indentUnit.of(" ".repeat(spaces)), EditorState.tabSize.of(spaces)] : []
}

/** CodeMirror editor for a text file. Its content is read through ref; extensions must not change. */
export function CodeEditor({
  ref,
  value,
  filename,
  readOnly = false,
  extensions = none,
  onChange,
}: {
  ref: Ref<EditorHandle>
  value: string
  filename: string
  readOnly?: boolean
  extensions?: Extension
  onChange: () => void
}) {
  const parent = useRef<HTMLDivElement>(null)
  const view = useRef<EditorView>(null)
  // What the user chose for the editor, which changes without opening the file again.
  const { editorWrap, editorIndent, editorKeys } = useSettings().settings
  const dark = useCodeDark()
  const [parts] = useState(() => ({ dark: new Compartment(), wrap: new Compartment(), indent: new Compartment(), keys: new Compartment() }))
  const choices = useRef<Record<keyof typeof parts, Extension>>({ dark: darkness(dark), wrap: none, indent: none, keys: none })
  const changed = useRef(onChange)
  useEffect(() => {
    changed.current = onChange
  })
  // Loaded with the editor, so that saving checks at once.
  const check = useMemo(() => checkOf(filename), [filename])
  useImperativeHandle(
    ref,
    () => ({
      value: () => view.current?.state.doc.toString() ?? "",
      insert: (text) => {
        view.current?.dispatch(view.current.state.replaceSelection(text))
        view.current?.focus()
      },
      problem: async () => {
        const editor = view.current
        const found = editor && (await check)?.(editor)[0]
        return found ? { message: found.message, at: found.from, line: editor.state.doc.lineAt(found.from).number } : undefined
      },
      reveal: (at) => {
        view.current?.dispatch({ selection: { anchor: Math.min(at, view.current.state.doc.length) }, scrollIntoView: true })
        view.current?.focus()
      },
    }),
    [check],
  )

  useEffect(() => {
    const language = new Compartment()
    const editor = new EditorView({
      parent: parent.current!,
      doc: value,
      extensions: [
        // Vim's keys come first, so that they take keys before the others.
        parts.keys.of(choices.current.keys),
        basicSetup,
        keymap.of([indentWithTab]),
        language.of([]),
        theme,
        parts.dark.of(choices.current.dark),
        parts.wrap.of(choices.current.wrap),
        parts.indent.of(choices.current.indent),
        syntaxHighlighting(highlight),
        EditorView.updateListener.of((u) => u.docChanged && changed.current()),
        EditorView.contentAttributes.of({ "aria-label": i18next.t("Contents of {{name}}", { name: filename }) }),
        EditorState.readOnly.of(readOnly),
        translated(),
        extensions,
      ],
    })
    view.current = editor
    let open = true
    void languageOf(filename)?.then((l) => open && editor.dispatch({ effects: language.reconfigure(l) }))
    return () => {
      open = false
      editor.destroy()
    }
  }, [value, filename, readOnly, extensions, parts])

  useEffect(() => {
    let open = true
    const set = (part: keyof typeof parts, extension: Extension) => {
      choices.current[part] = extension
      view.current?.dispatch({ effects: parts[part].reconfigure(extension) })
    }
    set("dark", darkness(dark))
    set("wrap", editorWrap === "on" ? EditorView.lineWrapping : none)
    set("indent", indentation(editorIndent, filename))
    if (editorKeys === "vim") void vim().then((keys) => open && set("keys", keys))
    else set("keys", none)
    return () => {
      open = false
    }
  }, [dark, editorWrap, editorIndent, editorKeys, filename, parts])

  return (
    <div
      ref={parent}
      data-code
      className="h-[65vh] min-h-80 overflow-hidden rounded-xl shadow-xl ring-1 shadow-black/10 ring-black/5 focus-within:ring-2 focus-within:ring-ring dark:ring-white/10"
    />
  )
}
