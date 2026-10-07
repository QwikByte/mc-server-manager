import { indentWithTab } from "@codemirror/commands"
import { syntaxHighlighting } from "@codemirror/language"
import { Compartment, EditorState, type Extension } from "@codemirror/state"
import { EditorView, keymap } from "@codemirror/view"
import { basicSetup } from "codemirror"
import i18next from "i18next"
import { type Ref, useEffect, useImperativeHandle, useMemo, useRef } from "react"
import { checkOf, highlight, languageOf, theme, translated } from "./editor-setup"

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
        basicSetup,
        keymap.of([indentWithTab]),
        language.of([]),
        theme,
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
  }, [value, filename, readOnly, extensions])

  return (
    <div
      ref={parent}
      className="h-[65vh] min-h-80 overflow-hidden rounded-xl shadow-xl ring-1 shadow-black/10 ring-black/5 focus-within:ring-2 focus-within:ring-ring dark:ring-white/10"
    />
  )
}
