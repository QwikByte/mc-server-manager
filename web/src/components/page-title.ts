import { useMatch, useMatches } from "@tanstack/react-router"
import { t } from "i18next"
import { useEffect, useSyncExternalStore } from "react"

declare module "@tanstack/react-router" {
  interface StaticDataRouteOption {
    /** What the route shows, untranslated (msg), e.g. "Servers"; the browser's tab is titled after it. */
    title?: string
  }
}

// The names pages give themselves, e.g. that of their server, by the match of their route.
const names = new Map<string, string>()
const listeners = new Set<() => void>()
let version = 0

function changed() {
  version++
  for (const listener of listeners) listener()
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => void listeners.delete(listener)
}

/** Puts what a page shows, e.g. the name of its server, into the title of the browser's tab, before the title of its route. */
export function usePageName(name: string | undefined) {
  const id = useMatch({ strict: false, select: (match) => match.id })
  useEffect(() => {
    if (!name) return
    names.set(id, name)
    changed()
    return () => {
      names.delete(id)
      changed()
    }
  }, [id, name])
}

/** Titles the browser's tab after the open page, from the inside out, e.g. "Files · lobby · Servers · Noryx". */
export function DocumentTitle() {
  useSyncExternalStore(subscribe, () => version)
  const matches = useMatches()
  const title = [
    ...matches.flatMap((match) => [match.staticData.title && t(match.staticData.title), names.get(match.id)]).reverse(),
    t("Noryx"),
  ]
    .filter(Boolean)
    .join(" · ")
  useEffect(() => {
    document.title = title
  }, [title])
  return null
}
