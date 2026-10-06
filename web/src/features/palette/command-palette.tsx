import { MagnifyingGlassIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { lazy, Suspense, useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

// The palette loads once the panel idles, which keeps its first load small. Once loaded, it
// shows without Suspense, which reveals it only after a moment, so that what is typed right
// after Ctrl+K reaches its search.
let Loaded: typeof import("./palette").Palette | undefined
const load = () => import("./palette").then((m) => (Loaded = m.Palette))
const Lazy = lazy(() => load().then((Palette) => ({ default: Palette })))

const mac = /mac|iphone|ipad/i.test(navigator.userAgent)

/** Opens the palette with Ctrl+K or ⌘K anywhere in the panel, or with its button; a folded one only shows its icon. */
export function PaletteButton({ className, onOpen, folded }: { className?: string; onOpen?: () => void; folded?: boolean }) {
  const [open, setOpen] = useState(false)
  useEffect(() => {
    const preload = () => void load()
    if ("requestIdleCallback" in window) requestIdleCallback(preload)
    else setTimeout(preload, 1000)
  }, [])
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key.toLowerCase() === "k" && (e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey) {
        e.preventDefault()
        setOpen((o) => !o)
      }
    }
    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [])

  return (
    <>
      <Button
        variant="outline"
        className={cn("justify-start text-muted-foreground", className)}
        aria-keyshortcuts={mac ? "Meta+K" : "Control+K"}
        onClick={() => {
          onOpen?.()
          setOpen(true)
        }}
      >
        <MagnifyingGlassIcon />
        <span className={cn("flex-1 text-left max-md:sr-only", folded && "md:sr-only")}>{t("Search…")}</span>
        <kbd className={cn("rounded border bg-muted px-1.5 font-mono text-[0.6875rem] max-md:hidden", folded && "md:hidden")}>{mac ? "⌘K" : "Ctrl K"}</kbd>
      </Button>
      {open &&
        (Loaded ? (
          <Loaded onClose={() => setOpen(false)} />
        ) : (
          <Suspense>
            <Lazy onClose={() => setOpen(false)} />
          </Suspense>
        ))}
    </>
  )
}
