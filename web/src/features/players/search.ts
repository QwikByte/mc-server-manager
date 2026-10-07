import { type Order, orders } from "@/lib/sort"

export type OnlineSort = "name" | "server" | "network"
export type ListSort = "name" | "servers"

/** The columns that sort the players online and the lists, with the direction each starts in. */
export const onlineSorts: Record<OnlineSort, Order> = { name: "asc", server: "asc", network: "asc" }
export const listSorts: Record<ListSort, Order> = { name: "asc", servers: "desc" }
const sorts = Object.keys({ ...onlineSorts, ...listSorts }) as (OnlineSort | ListSort)[]

/** The search of the players page, in the address so that it can be shared and bookmarked. */
export interface PlayerSearch {
  q?: string
  tab?: "banned" | "whitelisted" | "operators"
  network?: string
  sort?: OnlineSort | ListSort
  order?: Order
}

export function validatePlayerSearch(search: Record<string, unknown>): PlayerSearch {
  const text = (v: unknown) => (typeof v === "string" && v ? v : undefined)
  const tab = text(search.tab)
  return {
    q: text(search.q),
    tab: tab === "banned" || tab === "whitelisted" || tab === "operators" ? tab : undefined,
    network: text(search.network),
    sort: sorts.find((s) => s === search.sort),
    order: orders.find((o) => o === search.order),
  }
}
