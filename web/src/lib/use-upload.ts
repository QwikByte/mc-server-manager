import { useBlocker } from "@tanstack/react-router"
import { useRef, useState } from "react"

/**
 * Runs one upload at a time with its progress, from 0 to 1 while it runs, and a way to cancel it. Leaving the panel
 * asks first while it runs, as that would cancel it.
 */
export function useUpload() {
  const [progress, setProgress] = useState<number>()
  const controller = useRef<AbortController>(undefined)
  const uploading = progress !== undefined
  useBlocker({ shouldBlockFn: () => false, enableBeforeUnload: () => uploading })

  /** Runs an upload; a cancelled one ends without a result. */
  async function run<T>(upload: (onProgress: (fraction: number) => void, signal: AbortSignal) => Promise<T>): Promise<T | undefined> {
    const current = new AbortController()
    controller.current = current
    setProgress(0)
    try {
      return await upload(setProgress, current.signal)
    } catch (e) {
      if (current.signal.aborted) return undefined
      throw e
    } finally {
      setProgress(undefined)
    }
  }

  return { progress, uploading, run, cancel: () => controller.current?.abort() }
}
