import { Decoration, type DecorationSet, EditorView, MatchDecorator, ViewPlugin, type ViewUpdate } from "@codemirror/view"

// Variables, which the master fills in for each server, and secrets, which only the agents
// fill in, stand out in the editor.
const decorator = new MatchDecorator({
  regexp: /\{\{((?:server|network)\.[^{}\s]*|secret:[^{}\n]*)\}\}/g,
  decoration: (m) => Decoration.mark({ class: m[1].startsWith("secret:") ? "cm-placeholder-secret" : "cm-placeholder-variable" }),
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
    ".cm-placeholder-variable": { color: "var(--code-keyword)", backgroundColor: "rgb(255 255 255 / 0.08)" },
    ".cm-placeholder-secret": { color: "var(--code-string)", backgroundColor: "rgb(252 211 77 / 0.14)" },
  }),
]
