import { Decoration, type DecorationSet, EditorView, MatchDecorator, ViewPlugin, type ViewUpdate } from "@codemirror/view"
import { hidden, placeholderPattern } from "./api"

// Variables and connections to databases, which the master fills in for each server, and
// secrets and passwords, which only the agents fill in, stand out in the editor.
const decorator = new MatchDecorator({
  regexp: placeholderPattern,
  decoration: (m) => Decoration.mark({ class: hidden(m[1]) ? "cm-placeholder-secret" : "cm-placeholder-variable" }),
})

/** Highlights the placeholders of a file of a set. */
export const placeholders = [
  ViewPlugin.fromClass(
    class {
      decorations: DecorationSet
      constructor(view: EditorView) {
        this.decorations = decorator.createDeco(view)
      }
      update(u: ViewUpdate) {
        this.decorations = decorator.updateDeco(u, this.decorations)
      }
    },
    { decorations: (v) => v.decorations },
  ),
  EditorView.baseTheme({
    ".cm-placeholder-variable, .cm-placeholder-secret": { borderRadius: "3px", padding: "0 1px" },
    ".cm-placeholder-variable": { color: "var(--code-keyword)", backgroundColor: "color-mix(in srgb, var(--console-overlay) 8%, transparent)" },
    ".cm-placeholder-secret": { color: "var(--code-string)", backgroundColor: "rgb(252 211 77 / 0.14)" },
  }),
]
