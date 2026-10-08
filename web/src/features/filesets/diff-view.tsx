import { unifiedMergeView } from "@codemirror/merge"
import { syntaxHighlighting } from "@codemirror/language"
import { Compartment, EditorState } from "@codemirror/state"
import { EditorView, lineNumbers } from "@codemirror/view"
import { useEffect, useRef } from "react"
import { darkness, highlight, languageOf, theme, translated } from "@/features/files/editor-setup"
import { useCodeDark } from "@/features/preferences/api"
import { placeholders } from "./placeholders"

/** Shows how a text changes, read-only, with the unchanged parts collapsed. */
export function DiffView({ before, after, filename, label }: { before: string; after: string; filename: string; label: string }) {
  const parent = useRef<HTMLDivElement>(null)
  const dark = useCodeDark()
  useEffect(() => {
    const language = new Compartment()
    const view = new EditorView({
      parent: parent.current!,
      doc: after,
      extensions: [
        lineNumbers(),
        language.of([]),
        theme,
        darkness(dark),
        EditorView.theme({ "&": { height: "auto", maxHeight: "24rem" }, ".cm-scroller": { overflow: "auto" } }),
        syntaxHighlighting(highlight),
        placeholders,
        EditorState.readOnly.of(true),
        EditorView.editable.of(false),
        EditorView.contentAttributes.of({ "aria-label": label }),
        translated(),
        unifiedMergeView({ original: before, mergeControls: false, highlightChanges: true, collapseUnchanged: { margin: 3, minSize: 6 } }),
      ],
    })
    let open = true
    void languageOf(filename)?.then((l) => open && view.dispatch({ effects: language.reconfigure(l) }))
    return () => {
      open = false
      view.destroy()
    }
  }, [before, after, filename, label, dark])
  return <div ref={parent} data-code className="overflow-hidden rounded-lg ring-1 ring-black/5 dark:ring-white/10" />
}
