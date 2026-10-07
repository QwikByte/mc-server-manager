import type { KeyboardEvent } from "react"

// Where a key moves to from the radio at index of count, around the ends.
const moves: Record<string, (index: number, count: number) => number> = {
  ArrowLeft: (index, count) => (index - 1 + count) % count,
  ArrowUp: (index, count) => (index - 1 + count) % count,
  ArrowRight: (index, count) => (index + 1) % count,
  ArrowDown: (index, count) => (index + 1) % count,
  Home: () => 0,
  End: (_, count) => count - 1,
}

/**
 * The props of the buttons of a radiogroup, by their value, as in the ARIA pattern: Tab reaches
 * only the chosen one, or the first while none is, and the arrow keys, Home and End choose another.
 */
export function radios<T>(values: readonly T[], value: T, onChange: (value: T) => void) {
  const current = Math.max(0, values.indexOf(value))
  return (option: T) => {
    const index = values.indexOf(option)
    return {
      type: "button",
      role: "radio",
      "aria-checked": option === value,
      tabIndex: index === current ? 0 : -1,
      onClick: () => onChange(option),
      onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
        const to = moves[event.key]?.(index, values.length)
        if (to === undefined || event.altKey || event.ctrlKey || event.metaKey) return
        event.preventDefault()
        onChange(values[to])
        event.currentTarget.closest('[role="radiogroup"]')?.querySelectorAll<HTMLElement>('[role="radio"]')[to]?.focus()
      },
    } as const
  }
}
