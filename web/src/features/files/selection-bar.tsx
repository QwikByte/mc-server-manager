import { DownloadSimpleIcon, FolderOpenIcon, TrashIcon, XIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import { useAccess } from "@/features/access/use-access"
import { archiveUrl, join, type ServerFiles, useChangeEach } from "./api"
import { MoveDialog } from "./move-dialog"

/** The first few of many names. */
function listNames(names: string[], max = 8) {
  const listed = names.slice(0, max).join(", ")
  return names.length > max ? `${listed} ${t("and {{count}} more", { count: names.length - max })}` : listed
}

/**
 * Acts on the chosen files and folders of a folder at once: downloads them as one ZIP archive,
 * moves or deletes them, each checked like a single one.
 */
export function SelectionBar({
  files,
  dir,
  names,
  all,
  onClear,
}: {
  files: ServerFiles
  dir: string
  names: string[]
  /** Whether all of the folder is chosen, which downloads as the folder. */
  all: boolean
  onClear: () => void
}) {
  const writable = useAccess().can("files.write", files.nodeId, files.serverId)
  const change = useChangeEach(files)
  const [dialog, setDialog] = useState<"move" | "delete">()

  // Awaited rather than with callbacks of mutate, which end with the bar once its entries are gone.
  async function remove() {
    const failed = await change.mutateAsync(names.map((name) => ({ action: "delete", path: join(dir, name) })))
    if (failed.size === 0) return void toast.success(t("Deleted {{count}} items", { count: names.length }))
    toast.error(t("{{failed}} of {{count}} items couldn't be deleted", { failed: failed.size, count: names.length }), {
      description: [...failed]
        .slice(0, 5)
        .map(([i, e]) => `${names[i]}: ${e.message}`)
        .join("; "),
    })
  }

  return (
    <div className="sticky bottom-4 z-20 mt-6 flex flex-wrap items-center gap-2 rounded-2xl bg-popover/90 px-3 py-2.5 shadow-2xl ring-1 ring-foreground/10 backdrop-blur-xl">
      <Button variant="ghost" size="icon-sm" aria-label={t("Clear the selection")} title={t("Clear the selection")} onClick={onClear}>
        <XIcon />
      </Button>
      <p className="mr-auto text-sm font-medium tabular-nums">{t("{{count}} selected", { count: names.length })}</p>
      <Button size="sm" variant="outline" asChild>
        <a href={archiveUrl(files, dir, all ? [] : names)} download>
          <DownloadSimpleIcon />
          {t("Download as ZIP")}
        </a>
      </Button>
      {writable && (
        <>
          <Button size="sm" variant="outline" disabled={change.isPending} onClick={() => setDialog("move")}>
            <FolderOpenIcon />
            {t("Move to…")}
          </Button>
          <Button size="sm" variant="outline" className="text-destructive" disabled={change.isPending} onClick={() => setDialog("delete")}>
            <TrashIcon />
            {change.isPending ? t("Deleting…") : t("Delete")}
          </Button>
        </>
      )}
      {dialog === "move" && <MoveDialog files={files} dir={dir} names={names} onClose={() => setDialog(undefined)} />}
      <ConfirmDialog
        open={dialog === "delete"}
        onOpenChange={(open) => !open && setDialog(undefined)}
        title={t("Delete {{count}} items?", { count: names.length })}
        description={t("{{names}} will be deleted, folders with everything in them. This can't be undone.", {
          names: listNames(names),
          count: names.length,
          defaultValue_one: "{{names}} will be deleted, and if it's a folder, everything in it. This can't be undone.",
        })}
        action={t("Delete")}
        destructive
        onConfirm={() => void remove()}
      />
    </div>
  )
}
