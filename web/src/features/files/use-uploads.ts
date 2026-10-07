import { useQueryClient } from "@tanstack/react-query"
import { useBlocker } from "@tanstack/react-router"
import { useRef, useState } from "react"
import { join, type ServerFiles, upload } from "./api"

export interface Upload {
  id: number
  name: string
  progress: number
  state: "queued" | "uploading" | "done" | "failed"
  error?: string
}

interface Job {
  id: number
  server: ServerFiles
  file: File
  path: string
  overwrite: boolean
  controller: AbortController
}

/** Uploads files one after another and tracks their progress. */
export function useUploads(s: ServerFiles) {
  const queryClient = useQueryClient()
  const [uploads, setUploads] = useState<Upload[]>([])
  const queue = useRef<Job[]>([])
  const jobs = useRef(new Map<number, Job>())
  const nextId = useRef(0)
  const running = useRef(false)
  const busy = uploads.some((u) => u.state === "queued" || u.state === "uploading")

  const update = (id: number, patch: Partial<Upload>) => setUploads((list) => list.map((u) => (u.id === id ? { ...u, ...patch } : u)))
  const remove = (id: number) => setUploads((list) => list.filter((u) => u.id !== id))

  // Leaving the panel would cancel running uploads; other pages of it don't.
  useBlocker({ shouldBlockFn: () => false, enableBeforeUnload: () => busy })

  async function run() {
    if (running.current) return
    running.current = true
    for (let job = queue.current.shift(); job; job = queue.current.shift()) {
      const { id } = job
      update(id, { state: "uploading" })
      try {
        await upload(job.server, job.path, job.file, {
          overwrite: job.overwrite,
          signal: job.controller.signal,
          onProgress: (progress) => update(id, { progress }),
        })
        update(id, { state: "done", progress: 1 })
        setTimeout(() => remove(id), 4000)
        void queryClient.invalidateQueries({ queryKey: ["files", job.server.nodeId, job.server.serverId] })
      } catch (e) {
        if (job.controller.signal.aborted) remove(id)
        else update(id, { state: "failed", error: (e as Error).message })
      } finally {
        jobs.current.delete(id)
      }
    }
    running.current = false
  }

  /** Queues files for upload into a folder. */
  function add(files: File[], dir: string, overwrite: boolean) {
    const added = files.map((file) => ({
      id: nextId.current++,
      server: s,
      file,
      path: join(dir, file.name),
      overwrite,
      controller: new AbortController(),
    }))
    for (const job of added) jobs.current.set(job.id, job)
    queue.current.push(...added)
    setUploads((list) => [...list, ...added.map(({ id, file }) => ({ id, name: file.name, progress: 0, state: "queued" as const }))])
    void run()
  }

  /** Cancels a queued or running upload, or dismisses a finished one. */
  function cancel(id: number) {
    queue.current = queue.current.filter((job) => job.id !== id)
    jobs.current.get(id)?.controller.abort()
    remove(id)
  }

  return { uploads, add, cancel }
}
