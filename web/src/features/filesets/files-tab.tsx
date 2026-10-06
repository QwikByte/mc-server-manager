import { ArrowSquareInIcon, FileIcon, FilePlusIcon, LockKeyIcon, PencilSimpleIcon, TrashIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { type FormEvent, useRef, useState } from "react"
import { Callout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { useAccess } from "@/features/access/use-access"
import { CodeEditor, type EditorHandle } from "@/features/files/code-editor"
import { readText } from "@/features/files/api"
import { allServersQuery } from "@/features/servers/api"
import { msg } from "@/lib/i18n"
import { placeholders as find, type SetFile } from "./api"
import { placeholders } from "./placeholders"

// An example in an empty field, which needs no translation.
const examplePath = "plugins/LuckPerms/config.yml"

/** The files of a set: a list of them, and an editor for the chosen one. */
export function FilesTab({
  files,
  editable,
  selected,
  onSelect,
  onChange,
}: {
  files: SetFile[]
  editable: boolean
  selected?: string
  onSelect: (path: string) => void
  onChange: (files: SetFile[]) => void
}) {
  const [dialog, setDialog] = useState<"add" | "rename" | "import">()
  const sorted = [...files].sort((a, b) => a.path.localeCompare(b.path))
  const file = files.find((f) => f.path === selected) ?? sorted[0]
  const update = (path: string, change: Partial<SetFile>) => onChange(files.map((f) => (f.path === path ? { ...f, ...change } : f)))
  const add = (next: SetFile) => {
    onChange([...files.filter((f) => f.path !== next.path), next])
    onSelect(next.path)
  }

  return (
    <div className="grid gap-4 lg:grid-cols-[18rem_minmax(0,1fr)]">
      <aside className="surface flex flex-col gap-2 self-start rounded-xl p-2">
        {editable && (
          <div className="flex gap-1.5 p-1">
            <Button size="sm" variant="outline" className="flex-1" onClick={() => setDialog("add")}>
              <FilePlusIcon />
              {t("New file")}
            </Button>
            <Button size="sm" variant="outline" className="flex-1" onClick={() => setDialog("import")}>
              <ArrowSquareInIcon />
              {t("Import")}
            </Button>
          </div>
        )}
        {sorted.length === 0 ? (
          <p className="px-3 py-6 text-center text-sm text-muted-foreground">{t("No files yet")}</p>
        ) : (
          <ul aria-label={t("Files of the set")} className="grid gap-0.5">
            {sorted.map((f) => {
              const slash = f.path.lastIndexOf("/")
              const secret = find(f.content).some((p) => /^(secret|datastore):/.test(p))
              return (
                <li key={f.path}>
                  <button
                    type="button"
                    aria-current={f.path === file?.path}
                    onClick={() => onSelect(f.path)}
                    className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-sm transition-colors outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring aria-[current=true]:bg-primary/10 aria-[current=true]:text-foreground"
                  >
                    <FileIcon className="size-4 shrink-0 text-muted-foreground" weight="duotone" />
                    <span className="min-w-0 flex-1 truncate font-mono text-xs" title={f.path}>
                      <span className="text-muted-foreground">{f.path.slice(0, slash + 1)}</span>
                      {f.path.slice(slash + 1)}
                    </span>
                    {secret && <LockKeyIcon aria-label={t("Holds secrets")} className="size-3.5 shrink-0 text-warning" weight="fill" />}
                  </button>
                </li>
              )
            })}
          </ul>
        )}
      </aside>

      {file ? (
        <div className="min-w-0 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="min-w-0 truncate font-mono text-sm font-medium" title={file.path}>
              {file.path}
            </h2>
            {editable && (
              <div className="flex flex-wrap items-center gap-2">
                <label className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Switch checked={file.onlyIfMissing} onCheckedChange={(onlyIfMissing) => update(file.path, { onlyIfMissing })} />
                  {t("Only if missing")}
                </label>
                <Button size="sm" variant="outline" onClick={() => setDialog("rename")}>
                  <PencilSimpleIcon />
                  {t("Rename")}
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                  onClick={() => onChange(files.filter((f) => f.path !== file.path))}
                >
                  <TrashIcon />
                  {t("Remove")}
                </Button>
              </div>
            )}
          </div>
          {file.onlyIfMissing && (
            <Callout>{t("Servers that have the file keep their own, e.g. as a plugin rewrites it. Changes here only reach servers without it.")}</Callout>
          )}
          <FileEditor
            key={file.path}
            file={file}
            readOnly={!editable}
            onChange={(content) => update(file.path, { content })}
          />
          <dl aria-label={t("Placeholders")} className="grid gap-x-4 gap-y-1 text-xs sm:grid-cols-[auto_1fr]">
            {hints.map(([placeholder, hint]) => (
              <div key={placeholder} className="contents">
                <dt className="font-mono">{placeholder}</dt>
                <dd className="text-muted-foreground">{t(hint)}</dd>
              </div>
            ))}
          </dl>
        </div>
      ) : (
        <EmptyState
          icon={FileIcon}
          tone="info"
          title={t("Add the files of the set")}
          description={t("Create them here, or import them from a server and replace what differs with placeholders.")}
        />
      )}

      {dialog === "add" && (
        <PathDialog
          title={t("New file")}
          files={files}
          onClose={() => setDialog(undefined)}
          onSubmit={(path) => add({ path, content: "", onlyIfMissing: false })}
        />
      )}
      {dialog === "rename" && file && (
        <PathDialog
          title={t("Rename {{path}}", { path: file.path })}
          initial={file.path}
          files={files.filter((f) => f.path !== file.path)}
          onClose={() => setDialog(undefined)}
          onSubmit={(path) => {
            onChange(files.map((f) => (f.path === file.path ? { ...f, path } : f)))
            onSelect(path)
          }}
        />
      )}
      {dialog === "import" && <ImportDialog initial={file?.path} onClose={() => setDialog(undefined)} onImport={add} />}
    </div>
  )
}

const hints = [
  ["{{server.name}}", msg("The server's name")],
  ["{{server.id}}", msg("Its ID")],
  ["{{server.port}}", msg("Its port")],
  ["{{network.server}}", msg("Its name in its network, e.g. lobby")],
  ["{{secret:<name>}}", msg("A secret of the set, which only the agent fills in. It hides the file from the file manager.")],
] as const

/** The editor of a file, which starts with its content and reports each change. */
function FileEditor({ file, readOnly, onChange }: { file: SetFile; readOnly: boolean; onChange: (content: string) => void }) {
  const [initial] = useState(file.content)
  const editor = useRef<EditorHandle>(null)
  return (
    <CodeEditor
      ref={editor}
      value={initial}
      filename={file.path}
      readOnly={readOnly}
      extensions={placeholders}
      onChange={() => onChange(editor.current?.value() ?? "")}
    />
  )
}

/** Checks a path of a file of a set like the master, as far as the panel can, and returns it cleaned. */
function cleanPath(input: string, files: SetFile[]): [string, string?] {
  const path = input.trim().replace(/^\/+/, "").replace(/\/+/g, "/")
  if (!path || path.endsWith("/")) return [path, t("Enter the path of a file, e.g. plugins/LuckPerms/config.yml.")]
  if (path.split("/").some((s) => s === ".." || s === ".") || path.includes("\\")) return [path, t("Enter a path inside the server folder.")]
  if (files.some((f) => f.path === path)) return [path, t("The set has this file already.")]
  return [path]
}

function PathDialog({
  title,
  initial = "",
  files,
  onClose,
  onSubmit,
}: {
  title: string
  initial?: string
  files: SetFile[]
  onClose: () => void
  onSubmit: (path: string) => void
}) {
  const [path, setPath] = useState(initial)
  const [error, setError] = useState<string>()
  function submit(event: FormEvent) {
    event.preventDefault()
    const [clean, problem] = cleanPath(path, files)
    if (problem) return setError(problem)
    onSubmit(clean)
    onClose()
  }
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
          </DialogHeader>
          <Field>
            <FieldLabel htmlFor="fileset-path">{t("Path in the server folder")}</FieldLabel>
            <Input
              id="fileset-path"
              className="font-mono"
              autoFocus
              required
              placeholder={examplePath}
              value={path}
              onChange={(e) => setPath(e.target.value)}
            />
            {error && <FieldError>{error}</FieldError>}
          </Field>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit">{t("Save")}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

/** Takes a file over from a server, as the file manager shows it: with its known secrets hidden. */
function ImportDialog({ initial = "", onClose, onImport }: { initial?: string; onClose: () => void; onImport: (file: SetFile) => void }) {
  const { can } = useAccess()
  const { data: servers } = useQuery(allServersQuery)
  const readable = (servers ?? []).filter((s) => can("files.read", s.nodeId, s.id))
  const [server, setServer] = useState("")
  const [path, setPath] = useState(initial)
  const [error, setError] = useState<string>()
  const [pending, setPending] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    const [clean, problem] = cleanPath(path, [])
    const s = readable.find((r) => `${r.nodeId}/${r.id}` === server)
    if (problem || !s) return setError(problem ?? t("Choose a server."))
    setPending(true)
    try {
      onImport({ path: clean, content: await readText({ nodeId: s.nodeId, serverId: s.id }, clean), onlyIfMissing: false })
      onClose()
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setPending(false)
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{t("Import a file from a server")}</DialogTitle>
            <DialogDescription>
              {t("The set gets the file as the file manager shows it. Replace passwords in it with secrets of the set before applying it.")}
            </DialogDescription>
          </DialogHeader>
          <Field>
            <FieldLabel htmlFor="import-server">{t("Server")}</FieldLabel>
            <Select value={server} onValueChange={setServer}>
              <SelectTrigger id="import-server" className="w-full">
                <SelectValue placeholder={t("Choose a server")} />
              </SelectTrigger>
              <SelectContent>
                {readable.map((s) => (
                  <SelectItem key={`${s.nodeId}/${s.id}`} value={`${s.nodeId}/${s.id}`}>
                    {s.name} <span className="text-muted-foreground">· {s.nodeName}</span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <Field>
            <FieldLabel htmlFor="import-path">{t("Path in the server folder")}</FieldLabel>
            <Input
              id="import-path"
              className="font-mono"
              required
              placeholder={examplePath}
              value={path}
              onChange={(e) => setPath(e.target.value)}
            />
            <FieldDescription>{t("A file the set has at this path is replaced.")}</FieldDescription>
            {error && <FieldError>{error}</FieldError>}
          </Field>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={pending}>
              <ArrowSquareInIcon />
              {t("Import")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
