import { ArrowUpIcon, FolderPlusIcon, UploadSimpleIcon, WarningIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { type AnchorHTMLAttributes, type DragEvent, Fragment, useRef, useState } from "react"
import { toast } from "sonner"
import { Callout, ErrorCallout } from "@/components/callout"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { IconTile } from "@/components/icon-tile"
import { Breadcrumb, BreadcrumbItem, BreadcrumbLink, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator } from "@/components/ui/breadcrumb"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import type { Server } from "@/features/servers/api"
import { formatBytes, formatDateTime } from "@/lib/format"
import { contentUrl, type FileEntry, filesQuery, join, maxEditableBytes, type ServerFiles, useChangeFiles } from "./api"
import { FileActions } from "./file-actions"
import { FileTypeIcon } from "./file-icon"
import { NameDialog } from "./name-dialog"
import { UploadList } from "./upload-list"
import { useUploads } from "./use-uploads"

/** Lists a folder of a server; files can be dropped onto it to upload them. */
export function FileBrowser({ files, path, server }: { files: ServerFiles; path: string; server: Server }) {
  const { data, isPending, error } = useQuery(filesQuery(files, path))
  const change = useChangeFiles(files)
  const { uploads, add, cancel } = useUploads(files)
  const [dragging, setDragging] = useState(false)
  const [creating, setCreating] = useState(false)
  const [replace, setReplace] = useState<File[]>([])
  const picker = useRef<HTMLInputElement>(null)
  const dragDepth = useRef(0)
  const folder = path.split("/").pop() || server.name

  // Files that exist already are only replaced after confirmation.
  function start(selected: File[]) {
    const existing = new Set(data?.files.map((f) => f.name))
    const fresh = selected.filter((f) => !existing.has(f.name))
    if (fresh.length) add(fresh, path, false)
    setReplace(selected.filter((f) => existing.has(f.name)))
  }

  function drop(event: DragEvent) {
    event.preventDefault()
    dragDepth.current = 0
    setDragging(false)
    const items = [...event.dataTransfer.items].filter((item) => item.kind === "file")
    const isFolder = (item: DataTransferItem) => item.webkitGetAsEntry()?.isDirectory
    if (items.some(isFolder)) toast.error("Folders can't be uploaded. Create the folder and upload the files inside it.")
    start(items.flatMap((item) => (isFolder(item) ? [] : (item.getAsFile() ?? []))))
  }

  function dragBy(step: number) {
    return (event: DragEvent) => {
      if (!event.dataTransfer.types.includes("Files")) return
      event.preventDefault()
      dragDepth.current += step
      setDragging(dragDepth.current > 0)
    }
  }

  return (
    <section
      aria-labelledby="files-heading"
      className="relative"
      onDragEnter={dragBy(1)}
      onDragLeave={dragBy(-1)}
      onDragOver={(e) => e.preventDefault()}
      onDrop={drop}
    >
      <h2 id="files-heading" className="sr-only">
        Files
      </h2>
      {server.state !== "stopped" && (
        <Callout tone="warning" icon={WarningIcon} className="mb-4">
          The server is running. Stop it before you replace worlds or plugins, as it may overwrite or still use them.
        </Callout>
      )}
      <UploadList uploads={uploads} onCancel={cancel} />
      <div className="surface overflow-hidden rounded-xl">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b bg-muted/30 px-4 py-3">
          <PathBreadcrumb files={files} path={path} root={server.name} />
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setCreating(true)}>
              <FolderPlusIcon />
              New folder
            </Button>
            <Button onClick={() => picker.current?.click()}>
              <UploadSimpleIcon />
              Upload
            </Button>
            <input
              ref={picker}
              type="file"
              multiple
              hidden
              onChange={(e) => {
                start([...(e.target.files ?? [])])
                e.target.value = ""
              }}
            />
          </div>
        </div>
        {isPending ? (
          <Skeleton className="m-4 h-64" />
        ) : error ? (
          <ErrorCallout error={error} className="m-4" />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead className="hidden w-28 text-right sm:table-cell">Size</TableHead>
                <TableHead className="hidden w-48 md:table-cell">Modified</TableHead>
                <TableHead className="w-12">
                  <span className="sr-only">Actions</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {path && (
                <TableRow>
                  <TableCell colSpan={4}>
                    <FolderLink
                      files={files}
                      path={path.split("/").slice(0, -1).join("/")}
                      className="inline-flex items-center gap-2.5 text-muted-foreground hover:text-foreground"
                    >
                      <ArrowUpIcon className="size-4" />
                      Parent folder
                    </FolderLink>
                  </TableCell>
                </TableRow>
              )}
              {data.files.length === 0 && (
                <TableRow>
                  <TableCell colSpan={4} className="py-14 text-center text-muted-foreground">
                    This folder is empty. Drop files here to upload them.
                  </TableCell>
                </TableRow>
              )}
              {data.files.map((entry) => (
                <TableRow key={entry.name}>
                  <TableCell className="max-w-0 w-full">
                    <EntryLink files={files} dir={path} entry={entry} />
                  </TableCell>
                  <TableCell className="hidden text-right text-muted-foreground tabular-nums sm:table-cell">
                    {entry.directory ? "–" : formatBytes(entry.size)}
                  </TableCell>
                  <TableCell className="hidden text-muted-foreground md:table-cell">{formatDateTime(entry.modified)}</TableCell>
                  <TableCell>
                    <FileActions files={files} dir={path} entry={entry} />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>
      {data?.truncated && <p className="mt-3 text-sm text-muted-foreground">This folder has more entries than can be listed here.</p>}
      {dragging && (
        <div className="pointer-events-none absolute inset-0 z-20 flex flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed border-primary bg-background/85 backdrop-blur-sm">
          <IconTile icon={UploadSimpleIcon} size="lg" />
          <p className="heading text-lg">Drop to upload to {folder}</p>
        </div>
      )}
      <NameDialog
        open={creating}
        onOpenChange={setCreating}
        title="New folder"
        label="Name"
        action="Create folder"
        onSubmit={(name) => change.mutateAsync({ action: "mkdir", path: join(path, name) })}
      />
      <ConfirmDialog
        open={replace.length > 0}
        onOpenChange={(open) => !open && setReplace([])}
        title={replace.length === 1 ? `Replace ${replace[0].name}?` : `Replace ${replace.length} files?`}
        description={`${replace.map((f) => f.name).join(", ")} already ${replace.length === 1 ? "exists" : "exist"} in ${folder}.`}
        action="Replace"
        destructive
        onConfirm={() => add(replace, path, true)}
      />
    </section>
  )
}

/** Opens a folder; other props, e.g. from a Slot, go to the link element. */
function FolderLink({
  files,
  path,
  ...props
}: { files: ServerFiles; path: string } & Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href">) {
  return (
    <Link
      {...props}
      to="/nodes/$nodeId/servers/$serverId/files"
      params={{ nodeId: files.nodeId, serverId: files.serverId }}
      search={{ path: path || undefined }}
    />
  )
}

/** Opens a folder, a text file in the editor, or downloads a large file. */
function EntryLink({ files, dir, entry }: { files: ServerFiles; dir: string; entry: FileEntry }) {
  const path = join(dir, entry.name)
  const content = (
    <>
      <FileTypeIcon entry={entry} />
      <span className="truncate">{entry.name}</span>
    </>
  )
  const className = "flex min-w-0 items-center gap-3 font-medium hover:text-primary"
  if (entry.directory)
    return (
      <FolderLink files={files} path={path} className={className}>
        {content}
      </FolderLink>
    )
  if (entry.size > maxEditableBytes)
    return (
      <a href={contentUrl(files, path)} download className={className} title="Too large for the editor, downloads the file">
        {content}
      </a>
    )
  return (
    <Link
      to="/nodes/$nodeId/servers/$serverId/files"
      params={{ nodeId: files.nodeId, serverId: files.serverId }}
      search={{ path: dir || undefined, edit: path }}
      className={className}
    >
      {content}
    </Link>
  )
}

function PathBreadcrumb({ files, path, root }: { files: ServerFiles; path: string; root: string }) {
  const parts = path ? path.split("/") : []
  const crumbs = [{ name: root, path: "" }, ...parts.map((name, i) => ({ name, path: parts.slice(0, i + 1).join("/") }))]
  return (
    <Breadcrumb className="min-w-0">
      <BreadcrumbList>
        {crumbs.map((crumb, i) => (
          <Fragment key={crumb.path}>
            {i > 0 && <BreadcrumbSeparator />}
            <BreadcrumbItem>
              {i === crumbs.length - 1 ? (
                <BreadcrumbPage className="font-medium">{crumb.name}</BreadcrumbPage>
              ) : (
                <BreadcrumbLink asChild>
                  <FolderLink files={files} path={crumb.path}>
                    {crumb.name}
                  </FolderLink>
                </BreadcrumbLink>
              )}
            </BreadcrumbItem>
          </Fragment>
        ))}
      </BreadcrumbList>
    </Breadcrumb>
  )
}
