import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
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

/** The lists of the game servers of a network, or of all game servers the user may see. */
export const playerListsQuery = (network?: string) =>
  queryOptions({
    queryKey: ["players", "lists", network ?? "all"],
    queryFn: () => api<PlayerLists>(network ? `/players/lists?network=${encodeURIComponent(network)}` : "/players/lists"),
    refetchInterval: 30_000, // the lists change in the game and through plugins too
  })

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
