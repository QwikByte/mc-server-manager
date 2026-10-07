import { msg } from "@/lib/i18n"
import type { FileEntry } from "./api"

export const sorts = {
  name: msg("Name"),
  size: msg("Largest first"),
  modified: msg("Newest first"),
} as const

export type Sort = keyof typeof sorts

// The order chosen last applies to every folder.
const sortKey = "noryx-file-sort"

export function storedSort(): Sort {
  try {
    const sort = localStorage.getItem(sortKey)
    return sort === "size" || sort === "modified" ? sort : "name"
  } catch {
    return "name" // e.g. with site data blocked
  }
}

export function storeSort(sort: Sort) {
  try {
    localStorage.setItem(sortKey, sort)
  } catch {
    // The order then lasts until the page is left.
  }
}

/** The entries whose names contain filter, folders first, sorted as sort says; ties are sorted by name. */
export function browse(entries: FileEntry[], sort: Sort, filter: string) {
  const figure: Record<Sort, (e: FileEntry) => number> = {
    name: () => 0,
    size: (e) => (e.directory ? 0 : -e.size),
    modified: (e) => -Date.parse(e.modified),
  }
  const needle = filter.trim().toLowerCase()
  return entries
    .filter((e) => e.name.toLowerCase().includes(needle))
    .sort(
      (a, b) =>
        Number(b.directory) - Number(a.directory) ||
        figure[sort](a) - figure[sort](b) ||
        a.name.localeCompare(b.name, undefined, { numeric: true }),
    )
}
