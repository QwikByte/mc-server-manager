import { t } from "i18next"
import { useState } from "react"
import { useNetworkOf } from "@/features/networks/servers"
import { useUsages } from "@/features/usage/api"
import { type NodeServer, serverKey } from "./api"
import { BulkBar } from "./bulk-bar"
import { type Facts, filterServers, groupServers, type Property, type ServerSearch, sortServers, type View } from "./browse"
import { ServerToolbar } from "./server-toolbar"
import { ServerGrid, ServerTable } from "./server-views"

// The view chosen last applies where the address names none.
const viewKey = "noryx-server-view"

function storedView(): View | undefined {
  try {
    const view = localStorage.getItem(viewKey)
    return view === "grid" || view === "table" ? view : undefined
  } catch {
    return undefined // e.g. with site data blocked
  }
}

/**
 * Lists servers as cards or a table, searched, filtered, sorted and grouped by the settings in
 * the address. Selected servers can be started, stopped, restarted, sent a command and tagged at once;
 * the selection only counts the servers that are listed.
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
  const { found, counts, total } = filterServers(servers, search, facts)
  const groups = groupServers(sortServers(found, search.sort, facts), search.group, facts)
  const view = search.view ?? storedView() ?? (servers.length > 12 ? "table" : "grid")
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
    if (c.view) {
      try {
        localStorage.setItem(viewKey, c.view)
      } catch {
        // Only the address then keeps the view.
      }
    }
    onSearch(c)
  }

  const props = {
    groups,
    facts,
    showNode: !hidden.includes("node"),
    selected: (s: NodeServer) => selection.has(serverKey(s)),
    onSelect: (list: NodeServer[], on: boolean) => setSelection((sel) => toggle(sel, list.map(serverKey), on)),
    collapsed: (g: { key: string }) => collapsed.has(`${search.group}/${g.key}`),
    onCollapse: (g: { key: string }) => setCollapsed((c) => toggle(c, [`${search.group}/${g.key}`], !c.has(`${search.group}/${g.key}`))),
  }

  return (
    <>
      <ServerToolbar
        servers={servers}
        search={search}
        onSearch={change}
        counts={counts}
        total={total}
        facts={facts}
        hidden={hidden}
        view={view}
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
