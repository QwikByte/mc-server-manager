import { useEffect, useState } from "react"

/**
 * Whether the console, the terminal or the editor fills the window, and a change of it. Esc leaves, unless something
 * in it used the key already, e.g. to close suggestions.
 */
export function useMaximized() {
  const [maximized, setMaximized] = useState(false)
  useEffect(() => {
    if (!maximized) return
    const leave = (e: KeyboardEvent) => e.key === "Escape" && !e.defaultPrevented && setMaximized(false)
    window.addEventListener("keydown", leave)
    return () => window.removeEventListener("keydown", leave)
  }, [maximized])
  return [maximized, setMaximized] as const
}

/** The classes of a surface that fills the window, over everything but dialogs, menus and toasts. */
export const maximizedClass = "fixed inset-0 z-40 flex flex-col rounded-none"
