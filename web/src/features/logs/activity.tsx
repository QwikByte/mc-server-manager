import { ScrollIcon } from "@phosphor-icons/react"
import { getRouteApi, Link } from "@tanstack/react-router"
import { t } from "i18next"
import { Section } from "@/components/section"
import { Button } from "@/components/ui/button"
import { useLiveLogs } from "./api"
import { LogList } from "./log-list"

const serverRoute = getRouteApi("/_app/nodes/$nodeId/servers/$serverId/activity")

/** The Activity tab of a server: what happened to it, as it happens. */
export function ServerActivityPage() {
  const { nodeId, serverId } = serverRoute.useParams()
  const filter = { node: nodeId, server: serverId }
  useLiveLogs(filter, true)
  return (
    <>
      <div className="mb-4 flex justify-end">
        <OpenInLog node={nodeId} server={serverId} />
      </div>
      <LogList filter={filter} live />
    </>
  )
}

/** The latest entries about a node and its servers. */
export function NodeActivity({ nodeId }: { nodeId: string }) {
  const filter = { node: nodeId }
  useLiveLogs(filter, true)
  return (
    <Section title={t("Activity")} actions={<OpenInLog node={nodeId} />}>
      <LogList filter={filter} live />
    </Section>
  )
}

function OpenInLog({ node, server }: { node: string; server?: string }) {
  return (
    <Button variant="outline" size="sm" asChild>
      <Link to="/logs" search={{ node, server }}>
        <ScrollIcon />
        {t("Open in the log")}
      </Link>
    </Button>
  )
}
