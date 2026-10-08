import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import type { ReactNode } from "react"
import { ErrorCallout } from "@/components/callout"
import { Markdown } from "@/components/markdown"
import { Pill } from "@/components/status"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { formatDate } from "@/lib/format"
import { changesQuery } from "./api"
import { ChannelPill } from "./channel-pill"

/**
 * What changed in the versions of a project that run on a server type and Minecraft version: those after from up to
 * to, e.g. between the installed version and an update, or the newest ones. Changelogs come from their authors and
 * are shown without HTML.
 */
export function ChangesDialog({
  open,
  onOpenChange,
  title,
  project,
  type,
  version,
  from,
  to,
  current,
  footer,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  project: string
  type: string
  version: string
  from?: string
  to?: string
  /** The ID of the installed version, which is marked. */
  current?: string
  footer?: ReactNode
}) {
  const { data, error, isPending } = useQuery({ ...changesQuery(project, type, version, from, to), enabled: open })
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{t("What the authors wrote about their versions, the newest first.")}</DialogDescription>
        </DialogHeader>
        <div className="-mx-1 grid max-h-[60vh] gap-6 overflow-y-auto px-1">
          {isPending ? (
            <Skeleton className="h-40 rounded-xl" />
          ) : error ? (
            <ErrorCallout error={error} retry={false} />
          ) : data.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("No suitable version.")}</p>
          ) : (
            data.map((change) => (
              <section key={change.id} className="grid gap-2">
                <h3 className="flex flex-wrap items-center gap-2 text-sm font-semibold">
                  <span className="font-mono">{change.number}</span>
                  <ChannelPill channel={change.channel} />
                  {change.id === current && <Pill tone="info">{t("Installed")}</Pill>}
                  <span className="ml-auto text-xs font-normal text-muted-foreground">{formatDate(change.published)}</span>
                </h3>
                {change.changelog.trim() ? (
                  <div className="min-w-0 rounded-lg bg-muted/40 p-3 break-words">
                    <Markdown text={change.changelog} />
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">{t("Its author didn't write what changed.")}</p>
                )}
              </section>
            ))
          )}
        </div>
        {footer && <DialogFooter>{footer}</DialogFooter>}
      </DialogContent>
    </Dialog>
  )
}
