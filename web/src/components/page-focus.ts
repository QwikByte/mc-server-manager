import { useMatches } from "@tanstack/react-router"
import { useEffect, useRef } from "react"

/** The id of the content of the panel's pages, where the skip link goes. */
export const contentId = "content"

export const content = () => document.getElementById(contentId)

// Whether a page opened while an overlay hid it from screen readers, e.g. the menu after a link in it.
let hidden = false

/** Focuses the heading of the page, or its content until the heading shows, once no overlay hides it. */
function focusPage() {
  const main = document.querySelector("main")
  hidden = !!main?.closest('[aria-hidden="true"]')
  // Unless the page focused something itself, e.g. the field of the sign-in page.
  const focused = main !== document.activeElement && main?.contains(document.activeElement)
  if (!main || hidden || focused) return
  const target = main.querySelector("h1") ?? main
  target.focus({ preventScroll: true })
}

/**
 * Moves focus to the heading of a page that opened, so that screen readers announce it. Not when the
 * panel loads, and not when only a tab or the search of a page changes; the tabs of a hub, e.g. of
 * the settings, are one page.
 */
export function PageFocus() {
  const page = useMatches({
    select: (matches) => {
      const match = matches[2] ?? matches[1]
      return match && `${match.routeId}${match.pathname}`
    },
  })
  const shown = useRef(page)
  useEffect(() => {
    if (page === shown.current) return
    const loaded = shown.current !== undefined
    shown.current = page
    if (loaded) focusPage()
  }, [page])
  return null
}

/** Lets the heading of a page that shows after the page opened, e.g. once its server loaded, take the focus from the content. */
export function takeFocus(heading: HTMLElement | null) {
  const main = content()
  if (heading && main && document.activeElement === main) heading.focus({ preventScroll: true })
}

/**
 * Lets an overlay that closes give focus back to what opened it only if the focus got lost with it,
 * not e.g. once a link in it opened a page, whose heading takes the focus instead.
 */
export function keepFocus(event: Event) {
  if (hidden) {
    event.preventDefault()
    focusPage()
  } else if (document.activeElement && document.activeElement !== document.body) {
    event.preventDefault()
  }
}
