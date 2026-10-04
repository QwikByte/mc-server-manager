import { CubeIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { Section } from "@/components/section"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { serversQuery } from "./api"
import type { ServerSearch } from "./browse"
import { CreateServerDialog } from "./create-server-dialog"
import { ServerBrowser } from "./server-browser"

/** The servers of a node, to search, filter, group and act on at once. */
export function ServerList({
  nodeId,
  nodeName,
  search,
  onSearch,
}: {
  nodeId: string
  nodeName: string
  search: ServerSearch
  onSearch: (change: Partial<ServerSearch>) => void
}) {
  const { can } = useAccess()
  const { data: servers, isPending, error } = useQuery(serversQuery(nodeId))
  const create = can("servers.create", nodeId) && <CreateServerDialog nodeId={nodeId} />

  return (
    <Section title={t("Servers")} actions={servers && servers.length > 0 && create}>
      {isPending ? (
        <Skeleton className="h-44 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : servers.length === 0 ? (
        <EmptyState icon={CubeIcon} title={t("No servers on this node")}>
          {create}
        </EmptyState>
      ) : (
        <ServerBrowser
          servers={servers.map((s) => ({ ...s, nodeId, nodeName }))}
          search={{ ...search, node: undefined }}
          onSearch={onSearch}
          hidden={["node"]}
        />
      )}
    </Section>
  )
}
