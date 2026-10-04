/** The search of the players page, in the address so that it can be shared and bookmarked. */
export interface PlayerSearch {
  q?: string
  tab?: "banned" | "whitelisted" | "operators"
  network?: string
}

export function validatePlayerSearch(search: Record<string, unknown>): PlayerSearch {
  const text = (v: unknown) => (typeof v === "string" && v ? v : undefined)
  const tab = text(search.tab)
  return {
    q: text(search.q),
    tab: tab === "banned" || tab === "whitelisted" || tab === "operators" ? tab : undefined,
    network: text(search.network),
  }
}
