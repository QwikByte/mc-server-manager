import { useSyncExternalStore } from "react"

export type Theme = "light" | "dark" | "system"

/** The colours of actions, links and focus, which index.css defines and checks for contrast. */
export const accents = ["emerald", "blue", "violet", "graphite"] as const
export type Accent = (typeof accents)[number]

/** How much space the panel leaves; compact fits more on the screen. */
export type Density = "comfortable" | "compact"

/** How the panel looks in this browser. */
export interface Look {
  theme: Theme
  accent: Accent
  density: Density
}

// Where the browser keeps each part, which public/theme.js applies on load; none means the first value.
const parts = {
  theme: { key: "theme", values: ["system", "light", "dark"] },
  accent: { key: "noryx-accent", values: accents },
  density: { key: "noryx-density", values: ["comfortable", "compact"] },
} as const

function read<K extends keyof Look>(part: K): Look[K] {
  const { key, values } = parts[part]
  try {
    const value = localStorage.getItem(key)
    return (values.find((v) => v === value) ?? values[0]) as Look[K]
  } catch {
    return values[0] as Look[K] // e.g. with site data blocked
  }
}

const media = matchMedia("(prefers-color-scheme: dark)")
const listeners = new Set<() => void>()
let current: Look = { theme: read("theme"), accent: read("accent"), density: read("density") }

// public/theme.js applies the look on load, before the first paint; this keeps it in sync afterwards.
function apply() {
  const root = document.documentElement
  root.classList.toggle("dark", current.theme === "dark" || (current.theme === "system" && media.matches))
  root.dataset.accent = current.accent
  root.dataset.density = current.density
}
apply()
media.addEventListener("change", apply)

/** Changes parts of the look and keeps them in this browser. */
export function setLook(change: Partial<Look>) {
  const next = { theme: change.theme ?? current.theme, accent: change.accent ?? current.accent, density: change.density ?? current.density }
  if (JSON.stringify(next) === JSON.stringify(current)) return
  current = next
  for (const part of Object.keys(parts) as (keyof Look)[]) {
    const { key, values } = parts[part]
    try {
      if (current[part] === values[0]) localStorage.removeItem(key)
      else localStorage.setItem(key, current[part])
    } catch {
      // The choice then only lasts until the page is reloaded.
    }
  }
  apply()
  for (const listener of listeners) listener()
}

export const setTheme = (theme: Theme) => setLook({ theme })

/** How the panel looks; the theme "system" follows the operating system. */
export function useLook(): Look {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
    () => current,
  )
}

/** The colour theme chosen by the user; "system" follows the operating system. */
export const useTheme = () => useLook().theme
