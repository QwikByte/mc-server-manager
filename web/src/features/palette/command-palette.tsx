import { MagnifyingGlassIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { lazy, Suspense, useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { useRememberOpened } from "./recent"
import { type Opens, mac, searchChord, useShortcuts } from "./shortcuts"
import { ShortcutsDialog } from "./shortcuts-dialog"

// The palette loads once the panel idles, which keeps its first load small. Once loaded, it
// shows without Suspense, which reveals it only after a moment, so that what is typed right
// after Ctrl+K reaches its search.
let Loaded: typeof import("./palette").Palette | undefined
const load = () => import("./palette").then((m) => (Loaded = m.Palette))
const Lazy = lazy(() => load().then((Palette) => ({ default: Palette })))
const CreateDialog = lazy(() => import("./create-dialog").then((m) => ({ default: m.CreateDialog })))

/**
 * Opens the palette with Ctrl+K or ⌘K anywhere in the panel, or with its button; a folded one only shows its icon.
 * It also follows the other keyboard shortcuts and shows the dialogs they and the palette open.
 */
export function PaletteButton({ className, onOpen, folded }: { className?: string; onOpen?: () => void; folded?: boolean }) {
  const [open, setOpen] = useState(false)
  const [dialog, setDialog] = useState<Exclude<Opens, "search">>()
  useRememberOpened()
  useShortcuts((what) => (what === "search" ? setOpen((o) => !o) : setDialog(what)))
  useEffect(() => {
    const preload = () => void load()
    if ("requestIdleCallback" in window) requestIdleCallback(preload)
    else setTimeout(preload, 1000)
  }, [])

  const palette = {
    onClose: () => setOpen(false),
    onOpen: (what: Exclude<Opens, "search">) => {
      setOpen(false)
      setDialog(what)
    },
  }
  const close = () => setDialog(undefined)

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
        <kbd className={cn("rounded border bg-muted px-1.5 font-mono text-[0.6875rem] max-md:hidden", folded && "md:hidden")}>{searchChord}</kbd>
      </Button>
      {open &&
        (Loaded ? (
          <Loaded {...palette} />
        ) : (
          <Suspense>
            <Lazy {...palette} />
          </Suspense>
        ))}
      {dialog === "shortcuts" && <ShortcutsDialog onClose={close} />}
      {dialog && dialog !== "shortcuts" && (
        <Suspense>
          <CreateDialog what={dialog} onClose={close} />
        </Suspense>
      )}
    </>
  )
}
