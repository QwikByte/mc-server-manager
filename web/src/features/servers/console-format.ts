/** Part of a line in one colour and formatting, as class names. */
export interface Run {
  text: string
  className?: string
}

// Minecraft's colours on the console's dark background; the darkest are lighter, so that they stay readable.
const colors: Partial<Record<string, string>> = {
  "0": "text-[#737b8c]",
  "1": "text-[#6f7bff]",
  "2": "text-[#3fbf3f]",
  "3": "text-[#2fc2c2]",
  "4": "text-[#e0524f]",
  "5": "text-[#c260c2]",
  "6": "text-[#ffaa00]",
  "7": "text-[#aaaaaa]",
  "8": "text-[#8a93a6]",
  "9": "text-[#7b7bff]",
  a: "text-[#55ff55]",
  b: "text-[#55ffff]",
  c: "text-[#ff6b6b]",
  d: "text-[#ff6bff]",
  e: "text-[#ffff55]",
  f: "text-white",
}
// Bold and italic text, and the lines of struck through (m) and underlined (n) text, which need one class together.
const formats: Partial<Record<string, string>> = {
  l: "font-bold",
  o: "italic",
  m: "line-through",
  n: "underline",
  mn: "[text-decoration-line:line-through_underline]",
}

/**
 * Splits a line into runs by Minecraft's codes, which the agent keeps for colours: a colour resets the
 * formatting, as in Minecraft, and § before anything else is text. The codes only become class names.
 */
export function runs(line: string): Run[] {
  const result: Run[] = []
  let color: string | undefined
  let format = ""
  for (const [i, part] of line.split("§").entries()) {
    const code = part.charAt(0).toLowerCase()
    let text = part.slice(1)
    if (i === 0) text = part
    else if (colors[code]) [color, format] = [code, ""]
    else if (code === "r") [color, format] = [undefined, ""]
    else if (code && "lomn".includes(code)) format += format.includes(code) ? "" : code
    else text = `§${part}`
    if (!text) continue
    const lines = [..."mn"].filter((c) => format.includes(c)).join("")
    const classes = [color && colors[color], ...[..."lo"].map((c) => format.includes(c) && formats[c]), formats[lines]]
    result.push({ text, className: classes.filter(Boolean).join(" ") || undefined })
  }
  return result
}

/** Tells warnings and errors by the words Java's loggers and stack traces use. */
export function levelOf(text: string): "error" | "warn" | undefined {
  if (/\b(ERROR|SEVERE|FATAL)\b|Exception/.test(text)) return "error"
  if (/\bWARN(ING)?\b/.test(text)) return "warn"
  return undefined
}

/** Escapes text for a regular expression that finds it as it is. */
export const literal = (text: string) => text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
