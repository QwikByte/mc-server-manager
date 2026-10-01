import { type CSSProperties, useRef } from "react"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"

const colors: [code: string, name: string, hex: string][] = [
  ["0", "Black", "#000000"],
  ["1", "Dark blue", "#0000AA"],
  ["2", "Dark green", "#00AA00"],
  ["3", "Dark aqua", "#00AAAA"],
  ["4", "Dark red", "#AA0000"],
  ["5", "Dark purple", "#AA00AA"],
  ["6", "Gold", "#FFAA00"],
  ["7", "Gray", "#AAAAAA"],
  ["8", "Dark gray", "#555555"],
  ["9", "Blue", "#5555FF"],
  ["a", "Green", "#55FF55"],
  ["b", "Aqua", "#55FFFF"],
  ["c", "Red", "#FF5555"],
  ["d", "Light purple", "#FF55FF"],
  ["e", "Yellow", "#FFFF55"],
  ["f", "White", "#FFFFFF"],
]
const hex = Object.fromEntries(colors.map(([code, , value]) => [code, value]))
const formats: [code: string, label: string, style: CSSProperties][] = [
  ["l", "Bold", { fontWeight: 700 }],
  ["o", "Italic", { fontStyle: "italic" }],
  ["n", "Underline", { textDecoration: "underline" }],
  ["m", "Strikethrough", { textDecoration: "line-through" }],
]

/** Splits a line into runs with Minecraft's § formatting codes applied. */
function runs(line: string) {
  const result: { text: string; style: CSSProperties }[] = []
  let style: CSSProperties = {}
  for (const [i, part] of line.split("§").entries()) {
    const code = part[0]?.toLowerCase()
    let text = part
    if (i > 0 && code !== undefined) {
      text = part.slice(1)
      if (hex[code]) style = { color: hex[code] }
      else if (code === "r") style = {}
      else style = { ...style, ...formats.find(([c]) => c === code)?.[2] }
    }
    if (text) result.push({ text, style })
  }
  return result
}

/** MOTD with buttons that insert colour and format codes, and a preview. */
export function MotdField({
  id,
  value,
  onChange,
  disabled,
}: {
  id: string
  value: string
  onChange: (v: string) => void
  disabled?: boolean
}) {
  const input = useRef<HTMLTextAreaElement>(null)

  function insert(code: string) {
    const el = input.current
    const start = el?.selectionStart ?? value.length
    const end = el?.selectionEnd ?? value.length
    onChange(value.slice(0, start) + `§${code}` + value.slice(end))
    requestAnimationFrame(() => {
      el?.focus()
      el?.setSelectionRange(start + 2, start + 2)
    })
  }

  return (
    <div className="grid gap-2">
      <div className="flex flex-wrap items-center gap-1" role="toolbar" aria-label="Formatting">
        {colors.map(([code, name, color]) => (
          <button
            key={code}
            type="button"
            title={name}
            aria-label={`Insert colour ${name}`}
            disabled={disabled}
            onClick={() => insert(code)}
            className="size-6 rounded-md ring-1 ring-foreground/15 ring-inset outline-ring transition-transform hover:scale-110 focus-visible:outline-2 disabled:opacity-50 disabled:hover:scale-100"
            style={{ backgroundColor: color }}
          />
        ))}
        {formats.map(([code, label, style]) => (
          <Button
            key={code}
            type="button"
            size="icon-xs"
            variant="outline"
            title={label}
            aria-label={`Insert ${label}`}
            disabled={disabled}
            onClick={() => insert(code)}
          >
            <span style={style}>A</span>
          </Button>
        ))}
        <Button type="button" size="xs" variant="outline" disabled={disabled} onClick={() => insert("r")}>
          Reset
        </Button>
      </div>
      <Textarea
        id={id}
        ref={input}
        rows={2}
        className="min-h-0 resize-none font-mono"
        value={value}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value.split("\n").slice(0, 2).join("\n"))}
      />
      <div className="rounded-lg bg-console px-3 py-2.5 font-mono text-sm leading-5 text-[#AAAAAA]" aria-label="Preview">
        {value.split("\n").map((line, i) => (
          <div key={i} className="min-h-5 whitespace-pre-wrap">
            {runs(line).map((run, j) => (
              <span key={j} style={run.style}>
                {run.text}
              </span>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}
