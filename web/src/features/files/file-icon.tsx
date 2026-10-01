import { FileIcon, FileImageIcon, FileTextIcon, FileZipIcon, FolderIcon } from "@phosphor-icons/react"
import type { FileEntry } from "./api"

const kinds: Record<string, typeof FileIcon> = {
  jar: FileZipIcon,
  zip: FileZipIcon,
  gz: FileZipIcon,
  png: FileImageIcon,
  jpg: FileImageIcon,
}
for (const ext of ["txt", "log", "yml", "yaml", "json", "toml", "properties", "cfg", "conf", "sh", "js", "xml", "mcmeta", "secret"]) {
  kinds[ext] = FileTextIcon
}

export function FileTypeIcon({ entry }: { entry: FileEntry }) {
  const Icon = entry.directory ? FolderIcon : (kinds[entry.name.split(".").pop()?.toLowerCase() ?? ""] ?? FileIcon)
  return (
    <Icon
      className={entry.directory ? "size-4 shrink-0 text-primary" : "size-4 shrink-0 text-muted-foreground"}
      weight={entry.directory ? "fill" : "regular"}
    />
  )
}
