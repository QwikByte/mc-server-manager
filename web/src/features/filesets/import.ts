import type { QueryClient } from "@tanstack/react-query"
import { BinaryFileError, filesQuery, join, readText, type ServerFiles } from "@/features/files/api"
import type { Picked } from "@/features/files/path-picker"
import type { SetFile } from "./api"

/** Limits of a set that the master checks, which importing keeps to. */
export const maxFiles = 100
const maxFileBytes = 1 << 20
// Folders importing lists at most, so that picking a large one stays quick.
const maxFolders = 50

/**
 * Reads the text files among those picked and in the picked folders, at most max of them.
 * Binary and large files are skipped and counted, as are those beyond max.
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
      try {
        files.push({ path: item.path, content: await readText(server, item.path), onlyIfMissing: false })
      } catch (e) {
        if (!(e instanceof BinaryFileError)) throw e
        skipped++
      }
    }
  }
  return { files, skipped }
}
