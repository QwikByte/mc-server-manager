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
