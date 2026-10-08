import { CopyIcon, DotsThreeIcon, DownloadSimpleIcon, FileArchiveIcon, FolderOpenIcon, PencilSimpleIcon, TrashIcon } from "@phosphor-icons/react"
import { useQueryClient } from "@tanstack/react-query"
import { t } from "i18next"
import { useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import { useAccess } from "@/features/access/use-access"
import { useOperation } from "@/features/operations/use-operation"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { formatBytes } from "@/lib/format"
import { archiveUrl, contentUrl, extractArchive, type FileEntry, isArchive, join, type ServerFiles, useChangeFiles } from "./api"
import { CopyDialog } from "./copy-dialog"
import { ExtractDialog } from "./extract-dialog"
import { MoveDialog } from "./move-dialog"
import { NameDialog } from "./name-dialog"

/** Menu of a file or folder: download, extract an archive, rename, move, copy and delete. */
export function FileActions({ files, dir, entry }: { files: ServerFiles; dir: string; entry: FileEntry }) {
  const writable = useAccess().can("files.write", files.nodeId, files.serverId)
  const change = useChangeFiles(files)
  const operation = useOperation()
  const queryClient = useQueryClient()
  const [dialog, setDialog] = useState<"rename" | "move" | "copy" | "extract" | "delete">()
  const path = join(dir, entry.name)
  const close = (open: boolean) => !open && setDialog(undefined)

  const extract = (destination: string, overwrite: boolean) =>
    operation.run(
      (onStart) =>
        extractArchive(files, path, destination, overwrite, onStart).finally(
          () => void queryClient.invalidateQueries({ queryKey: ["files", files.nodeId, files.serverId] }),
        ),
      {
        title: t("Extracting {{name}}…", { name: entry.name }),
        notify: true,
        done: ({ files: count, size }) => ({
          message: t("Extracted {{count}} files from {{name}}", { count, name: entry.name, defaultValue_one: "Extracted {{count}} file from {{name}}" }),
          description: formatBytes(size),
        }),
      },
    )

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
              {!entry.directory && isArchive(entry.name) && (
                <DropdownMenuItem onSelect={() => setDialog("extract")}>
                  <FileArchiveIcon />
                  {t("Extract…")}
                </DropdownMenuItem>
              )}
              <DropdownMenuItem onSelect={() => setDialog("rename")}>
                <PencilSimpleIcon />
                {t("Rename")}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => setDialog("move")}>
                <FolderOpenIcon />
                {t("Move to…")}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => setDialog("copy")}>
                <CopyIcon />
                {t("Copy to…")}
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
      {dialog === "move" && <MoveDialog files={files} dir={dir} names={[entry.name]} onClose={() => setDialog(undefined)} />}
      {dialog === "copy" && <CopyDialog files={files} dir={dir} names={[entry.name]} onClose={() => setDialog(undefined)} />}
      {dialog === "extract" && <ExtractDialog dir={dir} name={entry.name} onClose={() => setDialog(undefined)} onExtract={extract} />}
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
