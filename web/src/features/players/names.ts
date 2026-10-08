import { useQuery } from "@tanstack/react-query"
import { useAccess } from "@/features/access/use-access"
import { maxPlayers, playerListsQuery, seenPlayersQuery, validPlayerName } from "./api"
import { useOnlinePlayers } from "./online"

/** Names of players typed so far: those added, and the text not added yet. */
export interface Names {
  names: string[]
  input: string
}

/** The names of players, those added and the valid ones of the text not added yet, each once. */
export function namesOf({ names, input }: Names) {
  return add(names, input).names
}

/** Adds the valid names of a text, separated by spaces or commas, and keeps the others as text. */
export function add(names: string[], text: string): Names {
  const next = [...names]
  const invalid: string[] = []
  for (const name of text.split(/[\s,;]+/).filter(Boolean)) {
    if (!validPlayerName(name)) invalid.push(name)
    else if (!next.some((n) => n.toLowerCase() === name.toLowerCase()) && next.length < maxPlayers) next.push(name)
  }
  return { names: next, input: invalid.join(" ") }
}

/**
 * The names of the players the servers know, to find the players of an action without typing their name: those
 * online, those seen and those who joined a server.
 */
export function useKnownNames(wanted = true) {
  const { canSomewhere } = useAccess()
  const enabled = wanted && canSomewhere("servers.view")
  const { players } = useOnlinePlayers(enabled)
  const { data: seen } = useQuery({ ...seenPlayersQuery(), enabled })
  const { data: lists } = useQuery({ ...playerListsQuery({ joined: true }), enabled })
  const names = new Map<string, string>()
  for (const name of [...players.map((p) => p.name), ...(seen?.players ?? []).map((p) => p.name), ...(lists?.joined ?? []).map((p) => p.name)]) {
    if (!names.has(name.toLowerCase())) names.set(name.toLowerCase(), name)
  }
  return [...names.values()]
}
