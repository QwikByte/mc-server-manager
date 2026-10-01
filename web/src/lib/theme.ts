import { useSyncExternalStore } from "react"

export type Theme = "light" | "dark" | "system"

const media = matchMedia("(prefers-color-scheme: dark)")
const listeners = new Set<() => void>()
let current = read()

function read(): Theme {
  try {
    const theme = localStorage.getItem("theme")
    return theme === "light" || theme === "dark" ? theme : "system"
  } catch {
    return "system"
  }
}

// public/theme.js applies the theme on load; this keeps it in sync afterwards.
function apply() {
  document.documentElement.classList.toggle("dark", current === "dark" || (current === "system" && media.matches))
}
media.addEventListener("change", apply)

export function setTheme(theme: Theme) {
  current = theme
  try {
    if (theme === "system") localStorage.removeItem("theme")
    else localStorage.setItem("theme", theme)
  } catch {
    // The choice then only lasts until the page is reloaded.
  }
  apply()
  for (const listener of listeners) listener()
}

/** The colour theme chosen by the user; "system" follows the operating system. */
export function useTheme(): Theme {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
    () => current,
  )
}
