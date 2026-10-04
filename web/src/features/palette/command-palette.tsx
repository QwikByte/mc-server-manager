import { MagnifyingGlassIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { lazy, Suspense, useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

// The palette loads when it first opens, which keeps the panel's first load small.
const Palette = lazy(() => import("./palette").then((m) => ({ default: m.Palette })))

const mac = /mac|iphone|ipad/i.test(navigator.userAgent)

/** Opens the palette with Ctrl+K or ⌘K anywhere in the panel, or with its button. */
export function PaletteButton({ className, onOpen }: { className?: string; onOpen?: () => void }) {
  const [open, setOpen] = useState(false)
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
        <span className="flex-1 text-left max-md:sr-only">{t("Search…")}</span>
        <kbd className="rounded border bg-muted px-1.5 font-mono text-[0.6875rem] max-md:hidden">{mac ? "⌘K" : "Ctrl K"}</kbd>
      </Button>
      {open && (
        <Suspense>
          <Palette onClose={() => setOpen(false)} />
        </Suspense>
      )}
    </>
  )
}
