import { DotsThreeIcon, DownloadSimpleIcon, PencilSimpleIcon, TrashIcon } from "@phosphor-icons/react"
import { useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
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
  const change = useChangeFiles(files)
  const [dialog, setDialog] = useState<"rename" | "delete">()
  const path = join(dir, entry.name)
  const close = (open: boolean) => !open && setDialog(undefined)

  return (
    <>
      {/* Not modal, so that focus moves to the dialogs opened from it. */}
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button size="icon-sm" variant="ghost" aria-label={`Actions for ${entry.name}`}>
            <DotsThreeIcon weight="bold" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem asChild>
            <a href={entry.directory ? archiveUrl(files, path) : contentUrl(files, path)} download>
              <DownloadSimpleIcon />
              {entry.directory ? "Download as ZIP" : "Download"}
            </a>
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => setDialog("rename")}>
            <PencilSimpleIcon />
            Rename
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive" onSelect={() => setDialog("delete")}>
            <TrashIcon />
            Delete
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <NameDialog
        open={dialog === "rename"}
        onOpenChange={close}
        title={`Rename ${entry.name}`}
        label="New name"
        initial={entry.name}
        action="Rename"
        onSubmit={(name) => change.mutateAsync({ action: "move", from: path, to: join(dir, name) })}
      />
      <ConfirmDialog
        open={dialog === "delete"}
        onOpenChange={close}
        title={`Delete ${entry.name}?`}
        description={
          entry.directory
            ? "The folder and everything in it will be deleted. This can't be undone."
            : "The file will be deleted. This can't be undone."
        }
        action="Delete"
        destructive
        onConfirm={() =>
          change.mutate(
            { action: "delete", path },
            { onSuccess: () => toast.success(`Deleted ${entry.name}`), onError: (e) => toast.error(e.message) },
          )
        }
      />
    </>
  )
}
