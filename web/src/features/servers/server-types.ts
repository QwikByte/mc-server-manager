import { ArrowsSplitIcon, CubeIcon } from "@phosphor-icons/react"
import type { Status } from "@/components/status"
import type { ServerState } from "./api"

export interface ServerType {
  value: string
  label: string
  proxy: boolean
}

export const serverTypes: ServerType[] = [
  { value: "paper", label: "Paper", proxy: false },
  { value: "purpur", label: "Purpur", proxy: false },
  { value: "vanilla", label: "Vanilla", proxy: false },
  { value: "fabric", label: "Fabric", proxy: false },
  { value: "forge", label: "Forge", proxy: false },
  { value: "neoforge", label: "NeoForge", proxy: false },
  { value: "velocity", label: "Velocity", proxy: true },
  { value: "bungeecord", label: "BungeeCord", proxy: true },
]

export const memoryOptionsMb = [512, 1024, 2048, 4096, 6144, 8192, 12288, 16384]

export function serverType(value: string): ServerType {
  return serverTypes.find((t) => t.value === value) ?? { value, label: value, proxy: false }
}

/** Sensible defaults: proxies listen on 25577 and need far less memory than game servers. */
export function defaults(type: string) {
  return serverType(type).proxy ? { port: 25577, memoryMb: 512 } : { port: 25565, memoryMb: 2048 }
}

/** The first free port from the preferred one on, within the node's port range. */
export function suggestPort(used: number[], preferred: number, min = 1024, max = 65535): number {
  const start = preferred >= min && preferred <= max ? preferred : min
  const free = (p: number) => !used.includes(p)
  for (let p = start; p <= max; p++) if (free(p)) return p
  for (let p = min; p < start; p++) if (free(p)) return p
  return start
}

/** Shows the version a server was created with; "LATEST" follows new releases. */
export function displayVersion(version: string) {
  return version === "LATEST" ? "latest" : version
}

export const serverStates: Record<ServerState, Status> = {
  running: { tone: "success", label: "Running" },
  starting: { tone: "warning", label: "Starting", pulse: true },
  stopped: { tone: "neutral", label: "Stopped" },
}

/** Game servers are emerald blocks, proxies violet forks. */
export function serverLook(type: string) {
  return serverType(type).proxy ? ({ icon: ArrowsSplitIcon, tone: "violet" } as const) : ({ icon: CubeIcon, tone: "success" } as const)
}
