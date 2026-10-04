import { t } from "i18next"
import type { Network } from "@/features/networks/api"
import type { ServerUsage } from "@/features/usage/api"
import { msg } from "@/lib/i18n"
import type { NodeServer, ServerState } from "./api"
import { serverType, states } from "./server-types"

export const sorts = {
  name: msg("Name"),
  state: msg("State"),
  players: msg("Players"),
  cpu: msg("CPU"),
  memory: msg("Memory"),
  node: msg("Node"),
} as const

export const groupings = {
  none: msg("No grouping"),
  network: msg("Network"),
  node: msg("Node"),
  type: msg("Type"),
  tag: msg("Tag"),
} as const

export type Sort = keyof typeof sorts
export type Grouping = keyof typeof groupings
/** What servers are filtered and grouped by. */
export type Property = Exclude<Grouping, "none">
export const properties: Property[] = ["network", "node", "type", "tag"]
export type View = "grid" | "table"

/** How the list of servers is filtered, sorted and shown, kept in the address to share and bookmark it. */
export interface ServerSearch {
  q?: string
  state?: ServerState
  type?: string
  node?: string
  /** The ID of a network, or "none" for servers outside of networks. */
  network?: string
  tag?: string
  sort?: Sort
  group?: Grouping
  view?: View
}

/** Reads the list's settings from the address. Each key must be set explicitly, see the login route. */
export function validateServerSearch(search: Record<string, unknown>): ServerSearch {
  const text = (key: string) => (typeof search[key] === "string" && search[key] ? search[key] : undefined)
  const pick = <T extends string>(key: string, allowed: readonly T[]) => allowed.find((v) => v === search[key])
  return {
    q: text("q"),
    state: pick("state", states),
    type: text("type"),
    node: text("node"),
    network: text("network"),
    tag: text("tag"),
    sort: pick("sort", Object.keys(sorts) as Sort[]),
    group: pick("group", Object.keys(groupings) as Grouping[]),
    view: pick("view", ["grid", "table"] as const),
  }
}

/** What the list knows about servers besides themselves. */
export interface Facts {
  usage: (s: NodeServer) => ServerUsage | undefined
  network: (s: NodeServer) => Network | undefined
}

/**
 * The values of a property of a server, with their labels: its network, node, type or tags.
 * Servers outside of networks have "none", those without tags "".
 */
export function valuesOf(property: Property, s: NodeServer, facts: Facts): [string, string][] {
  switch (property) {
    case "node":
      return [[s.nodeId, s.nodeName]]
    case "type":
      return [[s.type, serverType(s.type).label]]
    case "network": {
      const network = facts.network(s)
      return [network ? [network.id, network.name] : ["none", t("No network")]]
    }
    case "tag":
      return s.tags.length > 0 ? s.tags.map((tag) => [tag, `#${tag}`]) : [["", t("No tags")]]
  }
}

/** Whether a value stands for no network or no tags. */
const isNone = (value: string) => value === "" || value === "none"

/** The servers matching the search, and how many of those match each state regardless of the state filter. */
export function filterServers(servers: NodeServer[], search: ServerSearch, facts: Facts) {
  const words = search.q?.toLowerCase().split(/\s+/).filter(Boolean) ?? []
  const matching = servers.filter((s) => {
    const labels = properties.flatMap((p) =>
      valuesOf(p, s, facts)
        .filter(([v]) => !isNone(v))
        .map(([, label]) => label),
    )
    const text = [s.name, s.version, s.port, ...labels].join(" ").toLowerCase()
    return (
      words.every((w) => text.includes(w)) && properties.every((p) => !search[p] || valuesOf(p, s, facts).some(([v]) => v === search[p]))
    )
  })
  const counts = Object.fromEntries(states.map((state) => [state, matching.filter((s) => s.state === state).length])) as Record<
    ServerState,
    number
  >
  return { found: matching.filter((s) => !search.state || s.state === search.state), counts, total: matching.length }
}

/** Sorts by name, or by a figure with the largest first; ties are sorted by name. */
export function sortServers(servers: NodeServer[], sort: Sort = "name", facts: Facts) {
  const live = (s: NodeServer) => (facts.usage(s)?.running ? facts.usage(s) : undefined)
  const figure: Record<Sort, (s: NodeServer) => number | string> = {
    name: () => 0,
    state: (s) => states.indexOf(s.state),
    players: (s) => -(live(s)?.players?.online ?? -1),
    cpu: (s) => -(live(s)?.cpuMillis ?? -1),
    memory: (s) => -(live(s)?.memoryBytes ?? -1),
    node: (s) => s.nodeName.toLowerCase(),
  }
  const by = figure[sort]
  return [...servers].sort((a, b) => {
    const [x, y] = [by(a), by(b)]
    return (x < y ? -1 : x > y ? 1 : 0) || a.name.localeCompare(b.name, undefined, { numeric: true })
  })
}

export interface Group {
  key: string
  label: string
  servers: NodeServer[]
}

/** Splits sorted servers into groups sorted by name; those without a network or tag come last. With tags, a server is in the group of each of its tags. */
export function groupServers(servers: NodeServer[], grouping: Grouping = "none", facts: Facts): Group[] {
  if (grouping === "none") return [{ key: "", label: "", servers }]
  const groups = new Map<string, Group>()
  for (const s of servers) {
    for (const [key, label] of valuesOf(grouping, s, facts)) {
      if (!groups.has(key)) groups.set(key, { key, label, servers: [] })
      groups.get(key)!.servers.push(s)
    }
  }
  const last = (g: Group) => Number(isNone(g.key))
  return [...groups.values()].sort((a, b) => last(a) - last(b) || a.label.localeCompare(b.label, undefined, { numeric: true }))
}
