import type { Backend, ForcedHost, Forwarding, Network, NetworkSettings } from "./api"
import { key } from "./servers"

/**
 * The settings of a network while they are edited. Routes refer to servers by key rather than
 * by the names players use, so that renaming a server keeps its place in them.
 */
export interface Draft {
  name: string
  forwarding: Forwarding
  firewalled: boolean
  backends: Backend[]
  /** Keys of the servers players join and fall back to. */
  try: string[]
  /** Host names with the keys of their servers. */
  forcedHosts: ForcedHost[]
  bedrockPort: number
}

export function draftOf({ name, forwarding, firewalled, backends, try: tried, forcedHosts, bedrockPort }: NetworkSettings): Draft {
  const keyOf = (name: string) => {
    const b = backends.find((b) => b.name === name)
    return b ? key(b) : name
  }
  return {
    name,
    forwarding,
    firewalled,
    backends,
    try: tried.map(keyOf),
    forcedHosts: forcedHosts.map((h) => ({ ...h, servers: h.servers.map(keyOf) })),
    bedrockPort,
  }
}

export function settingsOf(d: Draft): NetworkSettings {
  const nameOf = (k: string) => d.backends.find((b) => key(b) === k)?.name ?? k
  return { ...d, try: d.try.map(nameOf), forcedHosts: d.forcedHosts.map((h) => ({ ...h, servers: h.servers.map(nameOf) })) }
}

/** Removes a server from the network and from its routes. */
export function withoutBackend(d: Draft, k: string): Draft {
  return {
    ...d,
    backends: d.backends.filter((b) => key(b) !== k),
    try: d.try.filter((s) => s !== k),
    forcedHosts: d.forcedHosts.map((h) => ({ ...h, servers: h.servers.filter((s) => s !== k) })),
  }
}

/** What saving the draft does to the servers of the network, for the operator to know before. */
export function effects(network: Network, d: Draft, bungee: boolean) {
  const before = new Map(network.backends.map((b) => [key(b), b]))
  const after = new Set(d.backends.map(key))
  const joined = d.backends.filter((b) => !before.has(key(b)))
  const left = network.backends.filter((b) => !after.has(key(b)))
  const renamed = d.backends.some((b) => before.has(key(b)) && before.get(key(b))?.name !== b.name)
  return {
    // Game servers restart when they join or leave, or when the forwarding changes or Bedrock
    // players start or stop joining, who can't sign their chat messages.
    restart: d.forwarding !== network.forwarding || !d.bedrockPort !== !network.bedrockPort ? d.backends : joined,
    left,
    // BungeeCord can't reload without a server it had.
    proxyRestarts: bungee && (left.length > 0 || renamed),
    // The proxy restarts to publish another Bedrock port and to load or unload Geyser.
    bedrock: d.bedrockPort === network.bedrockPort ? undefined : !d.bedrockPort ? "off" : !network.bedrockPort ? "on" : "port",
  }
}
