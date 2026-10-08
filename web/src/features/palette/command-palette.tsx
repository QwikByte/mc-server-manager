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
 * Opens the palette with Ctrl+K or ⌘K anywhere in the panel, or with its button, which shows only its icon on
 * small screens. It also follows the other keyboard shortcuts and shows the dialogs they and the palette open.
 */
export function PaletteButton({ className, onOpen }: { className?: string; onOpen?: () => void }) {
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
        className={cn(
          "h-8 justify-start bg-muted/50 font-normal text-muted-foreground shadow-none max-sm:w-8 max-sm:justify-center max-sm:px-0 sm:w-56 lg:w-72 dark:bg-muted/50",
          className,
        )}
        aria-keyshortcuts={mac ? "Meta+K" : "Control+K"}
        onClick={() => {
          onOpen?.()
          setOpen(true)
        }}
      >
        <MagnifyingGlassIcon />
        <span className="flex-1 text-left max-sm:sr-only">{t("Search…")}</span>
        <kbd className="rounded-sm border bg-card px-1.5 font-mono text-[0.6875rem] max-sm:hidden">{searchChord}</kbd>
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
