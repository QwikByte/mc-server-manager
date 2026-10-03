import { t } from "i18next"
import { type CSSProperties, useRef } from "react"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { msg } from "@/lib/i18n"

const colors: [code: string, name: string, hex: string][] = [
  ["0", msg("Black"), "#000000"],
  ["1", msg("Dark blue"), "#0000AA"],
  ["2", msg("Dark green"), "#00AA00"],
  ["3", msg("Dark aqua"), "#00AAAA"],
  ["4", msg("Dark red"), "#AA0000"],
  ["5", msg("Dark purple"), "#AA00AA"],
  ["6", msg("Gold"), "#FFAA00"],
  ["7", msg("Gray"), "#AAAAAA"],
  ["8", msg("Dark gray"), "#555555"],
  ["9", msg("Blue"), "#5555FF"],
  ["a", msg("Green"), "#55FF55"],
  ["b", msg("Aqua"), "#55FFFF"],
  ["c", msg("Red"), "#FF5555"],
  ["d", msg("Light purple"), "#FF55FF"],
  ["e", msg("Yellow"), "#FFFF55"],
  ["f", msg("White"), "#FFFFFF"],
]
const hex = Object.fromEntries(colors.map(([code, , value]) => [code, value]))
const formats: [code: string, label: string, style: CSSProperties][] = [
  ["l", msg("Bold"), { fontWeight: 700 }],
  ["o", msg("Italic"), { fontStyle: "italic" }],
  ["n", msg("Underline"), { textDecoration: "underline" }],
  ["m", msg("Strikethrough"), { textDecoration: "line-through" }],
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
      <div className="flex flex-wrap items-center gap-1" role="toolbar" aria-label={t("Formatting")}>
        {colors.map(([code, name, color]) => (
          <button
            key={code}
            type="button"
            title={t(name)}
            aria-label={t("Insert colour {{name}}", { name: t(name) })}
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
            title={t(label)}
            aria-label={t("Insert {{format}}", { format: t(label) })}
            disabled={disabled}
            onClick={() => insert(code)}
          >
            <span style={style}>A</span>
          </Button>
        ))}
        <Button type="button" size="xs" variant="outline" disabled={disabled} onClick={() => insert("r")}>
          {t("Reset")}
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
      <div className="rounded-lg bg-console px-3 py-2.5 font-mono text-sm leading-5 text-[#AAAAAA]" aria-label={t("Preview")}>
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
