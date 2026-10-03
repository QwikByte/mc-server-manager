import { DotsThreeIcon, DownloadSimpleIcon, PencilSimpleIcon, TrashIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import { useAccess } from "@/features/access/use-access"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { archiveUrl, contentUrl, type FileEntry, join, type ServerFiles, useChangeFiles } from "./api"
import { NameDialog } from "./name-dialog"

/** Menu of a file or folder: download, rename and delete. */
export function FileActions({ files, dir, entry }: { files: ServerFiles; dir: string; entry: FileEntry }) {
  const writable = useAccess().can("files.write", files.nodeId, files.serverId)
  const change = useChangeFiles(files)
  const [dialog, setDialog] = useState<"rename" | "delete">()
  const path = join(dir, entry.name)
  const close = (open: boolean) => !open && setDialog(undefined)

  return (
    <>
      {/* Not modal, so that focus moves to the dialogs opened from it. */}
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button size="icon-sm" variant="ghost" aria-label={t("Actions for {{name}}", { name: entry.name })}>
            <DotsThreeIcon weight="bold" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem asChild>
            <a href={entry.directory ? archiveUrl(files, path) : contentUrl(files, path)} download>
              <DownloadSimpleIcon />
              {entry.directory ? t("Download as ZIP") : t("Download")}
            </a>
          </DropdownMenuItem>
          {writable && (
            <>
              <DropdownMenuItem onSelect={() => setDialog("rename")}>
                <PencilSimpleIcon />
                {t("Rename")}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onSelect={() => setDialog("delete")}>
                <TrashIcon />
                {t("Delete")}
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <NameDialog
        open={dialog === "rename"}
        onOpenChange={close}
        title={t("Rename {{name}}", { name: entry.name })}
        label={t("New name")}
        initial={entry.name}
        action={t("Rename")}
        onSubmit={(name) => change.mutateAsync({ action: "move", from: path, to: join(dir, name) })}
      />
      <ConfirmDialog
        open={dialog === "delete"}
        onOpenChange={close}
        title={t("Delete {{name}}?", { name: entry.name })}
        description={
          entry.directory
            ? t("The folder and everything in it will be deleted. This can't be undone.")
            : t("The file will be deleted. This can't be undone.")
        }
        action={t("Delete")}
        destructive
        onConfirm={() =>
          change.mutate(
            { action: "delete", path },
            { onSuccess: () => toast.success(t("Deleted {{name}}", { name: entry.name })), onError: (e) => toast.error(e.message) },
          )
        }
      />
    </>
  )
}
