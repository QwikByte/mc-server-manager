import { ArrowSquareInIcon, CaretDownIcon, CaretRightIcon, FileIcon, FilePlusIcon, FolderIcon, LockKeyIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { useState } from "react"
import { Button } from "@/components/ui/button"
import { placeholders, type SetFile } from "./api"

/** A folder of the files of a set; a chain of folders without files of their own shows as one, e.g. plugins/LuckPerms. */
interface Folder {
  name: string
  path: string
  folders: Folder[]
  files: SetFile[]
}

function treeOf(files: SetFile[]): Folder {
  const root: Folder = { name: "", path: "", folders: [], files: [] }
  for (const file of files) {
    let folder = root
    for (const name of file.path.split("/").slice(0, -1)) {
      let next = folder.folders.find((f) => f.name === name)
      if (!next) {
        next = { name, path: folder.path ? `${folder.path}/${name}` : name, folders: [], files: [] }
        folder.folders.push(next)
      }
      folder = next
    }
    folder.files.push(file)
  }
  const compact = (f: Folder): Folder => {
    const [only] = f.folders
    if (f.path && f.files.length === 0 && f.folders.length === 1) return compact({ ...only, name: `${f.name}/${only.name}` })
    return { ...f, folders: f.folders.map(compact).sort((a, b) => a.name.localeCompare(b.name)), files: f.files.sort((a, b) => a.path.localeCompare(b.path)) }
  }
  return compact(root)
}

const row = "flex w-full items-center gap-1.5 rounded-lg py-1 pr-2 text-left text-sm transition-colors outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"

/** The files of a set as folders, with the actions that add files. */
export function FileTree({
  files,
  selected,
  editable,
  onSelect,
  onNew,
  onImport,
}: {
  files: SetFile[]
  selected?: string
  editable: boolean
  onSelect: (path: string) => void
  onNew: () => void
  onImport: () => void
}) {
  const [closed, setClosed] = useState<string[]>([])
  const toggle = (path: string) => setClosed(closed.includes(path) ? closed.filter((p) => p !== path) : [...closed, path])

  const render = (folder: Folder, depth: number) => (
    <ul className="grid gap-0.5">
      {folder.folders.map((f) => (
        <li key={f.path}>
          <button type="button" aria-expanded={!closed.includes(f.path)} onClick={() => toggle(f.path)} className={row} style={{ paddingLeft: 6 + depth * 14 }}>
            {closed.includes(f.path) ? <CaretRightIcon className="size-3 shrink-0" /> : <CaretDownIcon className="size-3 shrink-0" />}
            <FolderIcon className="size-4 shrink-0 text-warning" weight="fill" />
            <span className="min-w-0 truncate font-mono text-xs">{f.name}</span>
          </button>
          {!closed.includes(f.path) && render(f, depth + 1)}
        </li>
      ))}
      {folder.files.map((file) => (
        <li key={file.path}>
          <button
            type="button"
            aria-current={file.path === selected}
            onClick={() => onSelect(file.path)}
            className={`${row} aria-[current=true]:bg-primary/10 aria-[current=true]:text-foreground`}
            style={{ paddingLeft: 6 + depth * 14 + 18 }}
          >
            <FileIcon className="size-4 shrink-0 text-muted-foreground" weight="duotone" />
            <span className="min-w-0 flex-1 truncate font-mono text-xs" title={file.path}>
              {file.path.split("/").pop()}
            </span>
            {placeholders(file.content).some((p) => p.startsWith("secret:")) && (
              <LockKeyIcon aria-label={t("Holds secrets")} className="size-3.5 shrink-0 text-warning" weight="fill" />
            )}
          </button>
        </li>
      ))}
    </ul>
  )

  return (
    <aside aria-label={t("Files of the set")} className="surface flex flex-col gap-2 self-start rounded-xl p-2">
      {editable && (
        <div className="flex gap-1.5 p-1">
          <Button size="sm" variant="outline" className="flex-1" onClick={onNew}>
            <FilePlusIcon />
            {t("New file")}
          </Button>
          <Button size="sm" variant="outline" className="flex-1" onClick={onImport}>
            <ArrowSquareInIcon />
            {t("Import")}
          </Button>
        </div>
      )}
      {files.length === 0 ? <p className="px-3 py-6 text-center text-sm text-muted-foreground">{t("No files yet")}</p> : render(treeOf(files), 0)}
    </aside>
  )
}
