import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { ApiError, api } from "@/lib/api"

export interface FileEntry {
  name: string
  directory: boolean
  size: number
  modified: string
}

export interface Listing {
  files: FileEntry[]
  /** The folder has more entries than the agent lists. */
  truncated: boolean
}

/** Files of a server, addressed by the node and server they belong to. */
export interface ServerFiles {
  nodeId: string
  serverId: string
}

/** Files bigger than this are downloaded instead of opened in the editor. */
export const maxEditableBytes = 2 * 1024 ** 2

const base = ({ nodeId, serverId }: ServerFiles) => `/nodes/${nodeId}/servers/${serverId}/files`
const query = (path: string) => `?path=${encodeURIComponent(path)}`

/** Joins a folder and a name to a path, e.g. ("plugins", "x.jar") → "plugins/x.jar". */
export const join = (dir: string, name: string) => (dir ? `${dir}/${name}` : name)

export const contentUrl = (s: ServerFiles, path: string) => `/api${base(s)}/content${query(path)}`
export const archiveUrl = (s: ServerFiles, path: string) => `/api${base(s)}/archive${query(path)}`

export const filesQuery = (s: ServerFiles, path: string) =>
  queryOptions({
    queryKey: ["files", s.nodeId, s.serverId, path],
    queryFn: () => api<Listing>(`${base(s)}${query(path)}`),
  })

export type FileChange = { action: "mkdir" | "delete"; path: string } | { action: "move"; from: string; to: string }

export function useChangeFiles(s: ServerFiles) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (change: FileChange) => {
      switch (change.action) {
        case "mkdir":
          return api(`${base(s)}/directories`, { body: { path: change.path } })
        case "move":
          return api(`${base(s)}/move`, { body: { from: change.from, to: change.to } })
        case "delete":
          return api(`${base(s)}${query(change.path)}`, { method: "DELETE" })
      }
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["files", s.nodeId, s.serverId] }),
  })
}

export class BinaryFileError extends Error {}

/** Reads a file as UTF-8 text, or throws BinaryFileError for other content. */
export async function readText(s: ServerFiles, path: string): Promise<string> {
  const res = await fetch(contentUrl(s, path))
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new ApiError(res.status, data.error ?? `The file could not be loaded (status ${res.status}).`)
  }
  const bytes = new Uint8Array(await res.arrayBuffer())
  try {
    const text = new TextDecoder("utf-8", { fatal: true }).decode(bytes)
    if (!text.includes("\0")) return text
  } catch {
    // not UTF-8
  }
  throw new BinaryFileError("This file isn't text and can't be edited here.")
}

/**
 * Uploads a file with progress reports; fetch can't report upload progress. Without
 * overwrite, the master answers 409 if the file exists.
 */
export function upload(
  s: ServerFiles,
  path: string,
  body: Blob | string,
  { overwrite = false, onProgress, signal }: { overwrite?: boolean; onProgress?: (fraction: number) => void; signal?: AbortSignal } = {},
): Promise<FileEntry> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open("PUT", `${contentUrl(s, path)}${overwrite ? "&overwrite=true" : ""}`)
    xhr.setRequestHeader("Content-Type", "application/octet-stream")
    xhr.responseType = "json"
    xhr.upload.onprogress = (e) => e.lengthComputable && onProgress?.(e.loaded / e.total)
    xhr.onload = () =>
      xhr.status === 201
        ? resolve(xhr.response as FileEntry)
        : reject(new ApiError(xhr.status, xhr.response?.error ?? `The upload failed with status ${xhr.status}.`))
    xhr.onerror = () => reject(new Error("The connection to the panel was lost."))
    xhr.onabort = () => reject(new DOMException("The upload was cancelled.", "AbortError"))
    signal?.addEventListener("abort", () => xhr.abort())
    xhr.send(body)
  })
}
