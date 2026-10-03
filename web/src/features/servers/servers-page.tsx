import { CubeIcon, MagnifyingGlassIcon, WifiSlashIcon } from "@phosphor-icons/react"
import { useQueries, useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { useState } from "react"
import { Callout, ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
import { Skeleton } from "@/components/ui/skeleton"
import { nodesQuery } from "@/features/nodes/api"
import { usageQuery } from "@/features/usage/api"
import { allServersQuery } from "./api"
import { ServerCard } from "./server-list"
import { displayVersion, serverType } from "./server-types"

/** The servers the user may see on all nodes, with a search by name, type, version, port or node. */
export function ServersPage() {
  const { data: servers, isPending, error } = useQuery(allServersQuery)
  const { data: nodes } = useQuery(nodesQuery)
  const usages = useQueries({ queries: [...new Set(servers?.map((s) => s.nodeId))].map(usageQuery) })
  const usage = new Map(usages.flatMap((u) => u.data?.servers ?? []).map((u) => [u.id, u]))
  const [search, setSearch] = useState("")

  const words = search.toLowerCase().split(/\s+/).filter(Boolean)
  const found = servers
    ?.filter((s) => {
      const text = [s.name, serverType(s.type).label, displayVersion(s.version), s.port, s.nodeName].join(" ").toLowerCase()
      return words.every((w) => text.includes(w))
    })
    .sort((a, b) => a.name.localeCompare(b.name))
  const offline = nodes?.filter((n) => n.status === "offline").map((n) => n.name) ?? []

  return (
    <>
      <PageHeader
        icon={CubeIcon}
        title={t("Servers")}
        description={t("The servers on all nodes. They are created on the page of a node.")}
      />
      {offline.length > 0 && (
        <Callout tone="warning" icon={WifiSlashIcon} title={t("Some nodes are offline")} className="mb-6">
          {t("The servers on {{nodes}} aren't listed until they are back.", {
            nodes: offline.join(", "),
            count: offline.length,
            defaultValue_one: "The servers on {{nodes}} aren't listed until it is back.",
          })}
        </Callout>
      )}
      {isPending ? (
        <Skeleton className="h-44 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : servers.length === 0 ? (
        // Servers on offline nodes may exist; the notice above tells about them.
        nodes &&
        offline.length === 0 && (
          <EmptyState icon={CubeIcon} title={t("No servers yet")} description={t("Open a node to create a game server or a proxy on it.")}>
            <Button asChild>
              <Link to="/nodes">{t("Go to the nodes")}</Link>
            </Button>
          </EmptyState>
        )
      ) : (
        <>
          <InputGroup className="mb-6 w-full sm:max-w-xs">
            <InputGroupAddon>
              <MagnifyingGlassIcon />
            </InputGroupAddon>
            <InputGroupInput
              type="search"
              placeholder={t("Search servers")}
              aria-label={t("Search servers")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </InputGroup>
          {found?.length === 0 && <p className="text-sm text-muted-foreground">{t("No server matches your search.")}</p>}
          <ul className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {found?.map((server) => (
              <ServerCard key={server.id} nodeId={server.nodeId} nodeName={server.nodeName} server={server} usage={usage.get(server.id)} />
            ))}
          </ul>
        </>
      )}
    </>
  )
}
