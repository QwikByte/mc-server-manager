import { indentWithTab } from "@codemirror/commands"
import { syntaxHighlighting } from "@codemirror/language"
import { Compartment, EditorState, type Extension } from "@codemirror/state"
import { EditorView, keymap } from "@codemirror/view"
import { basicSetup } from "codemirror"
import i18next from "i18next"
import { type Ref, useEffect, useImperativeHandle, useRef } from "react"
import { highlight, languageOf, theme, translated } from "./editor-setup"

export interface EditorHandle {
  value: () => string
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
  useImperativeHandle(ref, () => ({ value: () => view.current?.state.doc.toString() ?? "" }), [])

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
