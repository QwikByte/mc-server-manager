import type { QueryClient } from "@tanstack/react-query"
import { t } from "i18next"
import { contentUrl, filesQuery, join, type ServerFiles } from "@/features/files/api"
import type { Picked } from "@/features/files/path-picker"
import { responseError } from "@/lib/api"
import type { SetFile } from "./api"
import { holdsCode, maxFileBytes, setFileOf } from "./binary"

/** Limits of a set that the master checks, which importing keeps to. */
export const maxFiles = 100
// Folders importing lists at most, so that picking a large one stays quick.
const maxFolders = 50

/**
 * Reads the files among those picked and in the picked folders, at most max of them, as text
 * or binary files. Large files and those with code are skipped and counted, as are those
 * beyond max.
 */
export async function importFiles(client: QueryClient, server: ServerFiles, picked: Picked[], max: number) {
  const files: SetFile[] = []
  const queue = [...picked]
  let skipped = 0
  let folders = 0
  for (let item = queue.shift(); item; item = queue.shift()) {
    if (item.directory) {
      if (++folders > maxFolders) continue
      const listing = await client.fetchQuery(filesQuery(server, item.path))
      const parent = item.path
      queue.push(...listing.files.map((e) => ({ path: join(parent, e.name), directory: e.directory, size: e.size })))
    } else if (files.length >= max || item.size > maxFileBytes) {
      skipped++
    } else {
      const res = await fetch(contentUrl(server, item.path))
      if (!res.ok) throw await responseError(res, t("The file could not be loaded (status {{status}}).", { status: res.status }))
      const bytes = new Uint8Array(await res.arrayBuffer())
      if (bytes.length > maxFileBytes || holdsCode(item.path, bytes)) skipped++
      else files.push(setFileOf(item.path, bytes))
    }
  }
  return { files, skipped }
}
