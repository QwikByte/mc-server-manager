import { useState } from "react"
import { useAccess } from "@/features/access/use-access"
import type { ServerRef } from "@/features/networks/api"
import type { Restart } from "./api"

export const batches = ["1", "2", "5", "10"] as const

/**
 * Whether to restart the running servers whose plugins change afterwards, and how many game servers of a network at a
 * time; request is what the master gets, which only asks for it if the user may restart all of them.
 */
export function useRestart(running: ServerRef[]) {
  const { can } = useAccess()
  const [on, setOn] = useState(false)
  const [batch, setBatch] = useState<(typeof batches)[number]>("1")
  const may = running.every((s) => can("servers.restart", s.nodeId, s.serverId))
  const request: Restart = on && may && running.length > 0 ? { restart: true, batch: Number(batch) } : {}
  return { running, on, setOn, batch, setBatch, may, request }
}
