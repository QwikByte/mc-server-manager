import { ArrowsSplitIcon, CubeIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import type { Status } from "@/components/status"
import { msg } from "@/lib/i18n"
import type { ServerState } from "./api"

export interface ServerType {
  value: string
  label: string
  proxy: boolean
  /** What the server loads from Modrinth, and the loaders those are made for; vanilla servers load nothing. */
  addons?: { kind: "plugins" | "mods"; loaders: string[] }
}

const bukkit = ["paper", "spigot", "bukkit"]

export const serverTypes: ServerType[] = [
  { value: "paper", label: "Paper", proxy: false, addons: { kind: "plugins", loaders: bukkit } },
  { value: "purpur", label: "Purpur", proxy: false, addons: { kind: "plugins", loaders: ["purpur", ...bukkit] } },
  { value: "vanilla", label: "Vanilla", proxy: false },
  { value: "fabric", label: "Fabric", proxy: false, addons: { kind: "mods", loaders: ["fabric"] } },
  { value: "forge", label: "Forge", proxy: false, addons: { kind: "mods", loaders: ["forge"] } },
  { value: "neoforge", label: "NeoForge", proxy: false, addons: { kind: "mods", loaders: ["neoforge"] } },
  { value: "velocity", label: "Velocity", proxy: true, addons: { kind: "plugins", loaders: ["velocity"] } },
  { value: "bungeecord", label: "BungeeCord", proxy: true, addons: { kind: "plugins", loaders: ["bungeecord", "waterfall"] } },
  { value: "waterfall", label: "Waterfall", proxy: true, addons: { kind: "plugins", loaders: ["waterfall", "bungeecord"] } },
]

export const memoryOptionsMb = [512, 1024, 2048, 4096, 6144, 8192, 12288, 16384]

export function serverType(value: string): ServerType {
  return serverTypes.find((type) => type.value === value) ?? { value, label: value, proxy: false }
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

/** Suggests a name for a copy, e.g. "Lobby" → "Lobby 2", "Lobby 2" → "Lobby 3". */
export function nextName(name: string, taken: string[]): string {
  const base = name.replace(/ \d+$/, "")
  for (let n = 2; ; n++) {
    const candidate = `${base.slice(0, 31 - String(n).length)} ${n}`
    if (!taken.includes(candidate)) return candidate
  }
}

/** Shows the version a server was created with; "LATEST" follows new releases. */
export function displayVersion(version: string) {
  return version === "LATEST" ? t("latest") : version
}

export const serverStates: Record<ServerState, Status> = {
  running: { tone: "success", label: msg("Running") },
  starting: { tone: "warning", label: msg("Starting"), pulse: true },
  crashing: { tone: "destructive", label: msg("Crashing"), pulse: true },
  stopped: { tone: "neutral", label: msg("Stopped") },
}

/** The states in the order lists show them. */
export const states = Object.keys(serverStates) as ServerState[]

/** Game servers are emerald blocks, proxies violet forks. */
export function serverLook(type: string) {
  return serverType(type).proxy ? ({ icon: ArrowsSplitIcon, tone: "violet" } as const) : ({ icon: CubeIcon, tone: "success" } as const)
}

/** Splits JVM options entered one per line. */
export const splitOptions = (lines: string) => lines.split(/\s+/).filter(Boolean)
