import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { t } from "i18next"
import { ApiError, api, responseError } from "@/lib/api"

export interface FileEntry {
  name: string
  directory: boolean
  size: number
  modified: string
  /** The file set that wrote the file, whose next apply replaces changes made here. */
  fileSet?: string
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

/** Downloads a folder as a ZIP archive, or only the files and folders in it with the given names. */
export const archiveUrl = (s: ServerFiles, path: string, names: string[] = []) =>
  `/api${base(s)}/archive${query(path)}${names.map((n) => `&name=${encodeURIComponent(n)}`).join("")}`

export const filesQuery = (s: ServerFiles, path: string) =>
  queryOptions({
    queryKey: ["files", s.nodeId, s.serverId, path],
    queryFn: () => api<Listing>(`${base(s)}${query(path)}`),
  })

export type FileChange = { action: "mkdir" | "delete"; path: string } | { action: "move"; from: string; to: string }

function changeFiles(s: ServerFiles, change: FileChange) {
  switch (change.action) {
    case "mkdir":
      return api(`${base(s)}/directories`, { body: { path: change.path } })
    case "move":
      return api(`${base(s)}/move`, { body: { from: change.from, to: change.to } })
    case "delete":
      return api(`${base(s)}${query(change.path)}`, { method: "DELETE" })
  }
}

export function useChangeFiles(s: ServerFiles) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (change: FileChange) => changeFiles(s, change),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["files", s.nodeId, s.serverId] }),
  })
}

/**
 * Makes changes one after another, each checked on its own like a single one, and returns the
 * errors of those that failed by their index.
 */
export function useChangeEach(s: ServerFiles) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (changes: FileChange[]) => {
      const failed = new Map<number, Error>()
      for (const [i, change] of changes.entries()) await changeFiles(s, change).catch((e: Error) => failed.set(i, e))
      return failed
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["files", s.nodeId, s.serverId] }),
  })
}

/** Creates folders with the folders they are in, unless they exist. */
export async function createFolders(s: ServerFiles, paths: string[]) {
  // Creating a folder creates those it is in.
  for (const path of new Set(paths.filter((p) => !paths.some((other) => other.startsWith(`${p}/`))))) {
    await changeFiles(s, { action: "mkdir", path }).catch((e) => {
      if (!(e instanceof ApiError && e.status === 409)) throw e
    })
  }
}

export class BinaryFileError extends Error {}

/** A text file, with the version it was read in if the agent tells it. */
export interface TextFile {
  text: string
  version?: string
}

/** Reads a file as UTF-8 text, or throws BinaryFileError for other content. */
export async function readText(s: ServerFiles, path: string): Promise<TextFile> {
  const res = await fetch(contentUrl(s, path))
  if (!res.ok) throw await responseError(res, t("The file could not be loaded (status {{status}}).", { status: res.status }))
  const bytes = new Uint8Array(await res.arrayBuffer())
  try {
    const text = new TextDecoder("utf-8", { fatal: true }).decode(bytes)
    if (!text.includes("\0")) return { text, version: res.headers.get("ETag") ?? undefined }
  } catch {
    // not UTF-8
  }
  throw new BinaryFileError(t("This file isn't text and can't be edited here."))
}

/** Thrown by saveText if the file changed on the server since it was read. */
export class FileChangedError extends Error {}

/**
 * Saves a text file and returns the version saved. With the version it was read in, it only
 * replaces that one, else throws FileChangedError; without, it replaces whatever is there.
 */
export async function saveText(s: ServerFiles, path: string, text: string, version?: string): Promise<string | undefined> {
  const res = await fetch(`${contentUrl(s, path)}&overwrite=true`, {
    method: "PUT",
    headers: { "Content-Type": "application/octet-stream", ...(version && { "If-Match": version }) },
    body: text,
  })
  if (res.status === 412) throw new FileChangedError((await responseError(res)).message)
  if (!res.ok) throw await responseError(res)
  return res.headers.get("ETag") ?? undefined
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
        : reject(new ApiError(xhr.status, xhr.response?.error ?? t("The upload failed with status {{status}}.", { status: xhr.status })))
    xhr.onerror = () => reject(new Error(t("The connection to the panel was lost.")))
    xhr.onabort = () => reject(new DOMException(t("The upload was cancelled."), "AbortError"))
    signal?.addEventListener("abort", () => xhr.abort())
    xhr.send(body)
  })
}
