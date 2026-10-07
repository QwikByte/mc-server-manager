import { t } from "i18next"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { useAccess } from "@/features/access/use-access"
import { shortcutGroups } from "./shortcuts"

/** The keys of a shortcut, typed one after the other. */
export function Keys({ keys }: { keys: readonly string[] }) {
  return (
    <span className="flex shrink-0 gap-1">
      {keys.map((key, i) => (
        <kbd key={i} className="min-w-5 rounded border bg-muted px-1.5 text-center font-mono text-[0.6875rem] text-muted-foreground">
          {key}
        </kbd>
      ))}
    </span>
  )
}

/** Lists the keyboard shortcuts the user may use. */
export function ShortcutsDialog({ onClose }: { onClose: () => void }) {
  const access = useAccess()
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("Keyboard shortcuts")}</DialogTitle>
          <DialogDescription>
            {t("Type the keys one after the other, e.g. g and then s for the servers. They do nothing while you type in a field.")}
          </DialogDescription>
        </DialogHeader>
        <div className="gap-x-10 sm:columns-2">
          {shortcutGroups(access).map((group) => (
            <section key={group.heading} className="mb-6 break-inside-avoid">
              <h3 className="mb-2 text-xs font-semibold tracking-wider text-muted-foreground uppercase">{t(group.heading)}</h3>
              <dl className="grid gap-2">
                {group.shortcuts.map((s) => (
                  <div key={s.keys.join(" ")} className="flex items-center justify-between gap-4">
                    <dt>{t(s.label)}</dt>
                    <dd>
                      <Keys keys={s.keys} />
                    </dd>
                  </div>
                ))}
              </dl>
            </section>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  )
}
