import { t } from "i18next"
import { useState } from "react"
import { useNetworkOf } from "@/features/networks/servers"
import { useSettings } from "@/features/preferences/api"
import { useUsages } from "@/features/usage/api"
import { sortingOf } from "@/lib/sort"
import { type NodeServer, serverKey } from "./api"
import { BulkBar } from "./bulk-bar"
import {
  type Facts,
  filterServers,
  groupServers,
  type Property,
  type ServerSearch,
  serverRows,
  sortOrders,
  sortServers,
} from "./browse"
import { ServerToolbar } from "./server-toolbar"
import { ServerGrid, ServerTable } from "./server-views"

/**
 * Lists servers as cards or a table, searched, filtered, sorted and grouped by the settings in
 * the address. Where it doesn't say, the view, sort and grouping chosen last apply, which the user
 * keeps in all lists and browsers. Selected servers can be started, stopped, restarted, sent a
 * command and tagged at once; the selection only counts the servers that are listed.
 */
export function ServerBrowser({
  servers,
  search,
  onSearch,
  hidden = [],
}: {
  servers: NodeServer[]
  search: ServerSearch
  onSearch: (change: Partial<ServerSearch>) => void
  hidden?: Property[]
}) {
  const usages = useUsages(servers.map((s) => s.nodeId))
  const networkOf = useNetworkOf()
  const facts: Facts = { usage: (s) => usages.server(s.nodeId, s.id), network: (s) => networkOf({ nodeId: s.nodeId, serverId: s.id }) }
  const { settings, change: changeSettings } = useSettings()
  // A list that doesn't offer the grouping chosen last, e.g. by node on a node's page, doesn't group.
  // The sort chosen last applies while the address names neither a column nor an order.
  const saved = search.sort === undefined && search.order === undefined
  const shown: ServerSearch = {
    ...search,
    sort: saved ? settings.serverSort : search.sort,
    order: saved ? settings.serverOrder : search.order,
    group: search.group ?? (hidden.includes(settings.serverGroup as Property) ? undefined : settings.serverGroup),
  }
  const { found, counts, total } = filterServers(servers, search, facts)
  const sorting = sortingOf(shown, sortOrders, (c) => {
    const by = c.sort ?? "name"
    changeSettings({ serverSort: by, serverOrder: c.order ?? sortOrders[by] })
    onSearch(c)
  })
  const groups = groupServers(sortServers(found, sorting.by, sorting.order, facts), shown.group, facts)
  const view = search.view ?? settings.serverView ?? (servers.length > 12 ? "table" : "grid")
  const [selection, setSelection] = useState(new Set<string>())
  const [collapsed, setCollapsed] = useState(new Set<string>())
  const selected = found.filter((s) => selection.has(serverKey(s)))
  const toggle = (set: Set<string>, keys: string[], on: boolean) => {
    const next = new Set(set)
    for (const k of keys) {
      if (on) next.add(k)
      else next.delete(k)
    }
    return next
  }

  function change(c: Partial<ServerSearch>) {
    if (c.view) changeSettings({ serverView: c.view })
    if ("group" in c) changeSettings({ serverGroup: c.group ?? "none" })
    onSearch(c)
  }

  const props = {
    groups,
    facts,
    sorting,
    showNode: !hidden.includes("node"),
    selected: (s: NodeServer) => selection.has(serverKey(s)),
    onSelect: (list: NodeServer[], on: boolean) => setSelection((sel) => toggle(sel, list.map(serverKey), on)),
    collapsed: (g: { key: string }) => collapsed.has(`${shown.group}/${g.key}`),
    onCollapse: (g: { key: string }) => setCollapsed((c) => toggle(c, [`${shown.group}/${g.key}`], !c.has(`${shown.group}/${g.key}`))),
  }

  return (
    <>
      <ServerToolbar
        servers={servers}
        search={shown}
        onSearch={change}
        counts={counts}
        total={total}
        facts={facts}
        hidden={hidden}
        view={view}
        sorting={sorting}
        // In the order shown, each server once, also when grouped by tags.
        rows={() => serverRows([...new Set(groups.flatMap((g) => g.servers))], facts)}
      />
      {found.length === 0 ? (
        <p className="rounded-xl bg-muted/50 p-6 text-center text-sm text-muted-foreground">{t("No server matches your search.")}</p>
      ) : view === "table" ? (
        <ServerTable {...props} />
      ) : (
        <ServerGrid {...props} />
      )}
      {selected.length > 0 && <BulkBar selected={selected} onClear={() => setSelection(new Set())} />}
    </>
  )
}
