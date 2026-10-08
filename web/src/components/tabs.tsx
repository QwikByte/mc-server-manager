import { LayoutGroup } from "motion/react"
import { type ReactNode, useEffect, useId, useRef, useState } from "react"

const fade = "2.5rem"

/**
 * Navigation between the parts of a page, which are child routes; its links are TabLinks. Tabs that don't fit
 * scroll, and fade out at the edge that has more of them; the active tab scrolls into view.
 */
export function Tabs({ label, children }: { label: string; children: ReactNode }) {
  const ref = useRef<HTMLElement>(null)
  const [more, setMore] = useState({ before: false, after: false })

  useEffect(() => {
    const nav = ref.current
    if (!nav) return
    const update = () => {
      const before = nav.scrollLeft > 1
      const after = nav.scrollLeft + nav.clientWidth < nav.scrollWidth - 1
      setMore((m) => (m.before === before && m.after === after ? m : { before, after }))
    }
    update()
    const observer = new ResizeObserver(update)
    observer.observe(nav)
    nav.addEventListener("scroll", update, { passive: true })
    return () => {
      observer.disconnect()
      nav.removeEventListener("scroll", update)
    }
  }, [])

  // Without scrollIntoView, which would scroll the page too.
  useEffect(() => {
    const nav = ref.current
    const active = nav?.querySelector<HTMLElement>('[aria-current="page"]')
    if (!nav || !active) return
    const left = active.offsetLeft - nav.offsetLeft
    if (left < nav.scrollLeft) nav.scrollLeft = left - 24
    else if (left + active.offsetWidth > nav.scrollLeft + nav.clientWidth) nav.scrollLeft = left + active.offsetWidth - nav.clientWidth + 24
  })

  const mask = `linear-gradient(to right, ${more.before ? `transparent, black ${fade}` : "black"}, ${more.after ? `black calc(100% - ${fade}), transparent` : "black"})`
  return (
    <div className="relative mb-7 shadow-[inset_0_-1px_0_var(--border)]">
      <nav
        ref={ref}
        aria-label={label}
        style={more.before || more.after ? { maskImage: mask } : undefined}
        className="flex max-w-full gap-0.5 overflow-x-auto [scrollbar-width:none]"
      >
        <LayoutGroup id={useId()}>{children}</LayoutGroup>
      </nav>
    </div>
  )
}
