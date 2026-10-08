import { keepPreviousData, queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import type { ServerRef } from "@/features/networks/api"
import { operate } from "@/features/operations/api"
import type { Followed } from "@/features/servers/api"
import { api } from "@/lib/api"

/** What can be done to a player on game servers; turning the whitelist on or off takes no player. */
export type PlayerAction =
  | "kick"
  | "ban"
  | "pardon"
  | "whitelist_add"
  | "whitelist_remove"
  | "op"
  | "deop"
  | "whitelist_on"
  | "whitelist_off"

/** How a change ended on a server. */
export interface PlayerResult extends ServerRef {
  /** The server doesn't run and changes the player once it does. */
  pending?: boolean
  output?: string
  error?: string
}

/** A player in the lists of servers. */
export interface Listed {
  name: string
  uuid?: string
  /** Why, since when, until when and by whom the player is banned, of the newest ban; until only for temporary ones. */
  reason?: string
  since?: string
  until?: string
  source?: string
  /** The servers whose list has the player. */
  servers: ServerRef[]
}

/** What the lists of a server don't tell. */
export interface ServerLists extends ServerRef {
  whitelistEnabled: boolean
  /** Changes that wait for the server to run. */
  pending: { action: PlayerAction; name?: string; reason?: string }[]
  /** Why the lists of the server are missing. */
  error?: string
}

/** The lists of servers, joined by player. */
export interface PlayerLists {
  servers: ServerLists[]
  banned: Listed[]
  whitelisted: Listed[]
  operators: Listed[]
}

/** The lists of the game servers of a network, of one game server, or of all game servers the user may see. */
export function playerListsQuery({ network, server }: { network?: string; server?: ServerRef } = {}) {
  const filter = new URLSearchParams(server ? { node: server.nodeId, server: server.serverId } : network ? { network } : {})
  return queryOptions({
    queryKey: ["players", "lists", filter.toString()],
    queryFn: () => api<PlayerLists>(`/players/lists?${filter}`),
    refetchInterval: 30_000, // the lists change in the game and through plugins too
  })
}

/** When a player was first and last seen online, and for how many minutes. */
export interface Span {
  firstSeen: string
  lastSeen: string
  minutes: number
}

/** A player seen online, with the servers, those seen last first. */
export interface SeenPlayer extends Span {
  name: string
  servers: (ServerRef & Span)[]
}

/** Where and when a player was online, with the minutes of each day (as 2026-10-01, in UTC). */
export interface PlayerHistory extends SeenPlayer {
  days: { day: string; minutes: number }[]
}

/** The players seen on the servers the user may see, of a network, of one server or all, those seen last first. */
export function seenPlayersQuery({ q = "", network, server, limit }: { q?: string; network?: string; server?: ServerRef; limit?: number } = {}) {
  const search = new URLSearchParams({
    q,
    ...(network && { network }),
    ...(server && { node: server.nodeId, server: server.serverId }),
    ...(limit && { limit: String(limit) }),
  })
  return queryOptions({
    queryKey: ["players", "seen", search.toString()],
    queryFn: () => api<{ players: SeenPlayer[]; total: number }>(`/players/seen?${search}`),
    placeholderData: keepPreviousData,
    refetchInterval: 60_000, // the master notes the players online every minute
  })
}

export const playerHistoryQuery = (name: string) =>
  queryOptions({
    queryKey: ["players", "seen", "player", name.toLowerCase()],
    queryFn: () => api<PlayerHistory>(`/players/seen/${encodeURIComponent(name)}`),
    refetchInterval: 60_000,
  })

/** Whether a name is that of a player that the master and the agents take: of Java players, or of Bedrock players with Floodgate's dot. */
export const validPlayerName = (name: string) => /^\.?[A-Za-z0-9_]{1,16}$/.test(name)

export interface PlayerChange {
  action: PlayerAction
  name?: string
  reason?: string
  servers: ServerRef[]
}

/** Changes a player on servers: at once on those that run, on the others once they do. */
export function useChangePlayer() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ onStart, ...change }: PlayerChange & Followed) =>
      operate<{ results: PlayerResult[] }>("/players/actions", { body: change }, onStart).then((r) => r.results),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["players"] }),
  })
}

/** Sends a player to another server of a network, through its proxy. */
export function useSendPlayer(networkId: string) {
  return useMutation({
    mutationFn: (body: { name: string; server: string }) => api(`/networks/${networkId}/players/send`, { body }),
  })
}

/** Sends the players of a game server of a network to another server of it, the one players join first if it runs. */
export function useMovePlayers(networkId: string) {
  return useMutation({
    mutationFn: (server: ServerRef) => api<{ players: number }>(`/networks/${networkId}/players/move`, { body: { server } }),
  })
}
