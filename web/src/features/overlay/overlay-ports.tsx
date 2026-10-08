import { CheckCircleIcon, DatabaseIcon, PlugsConnectedIcon, SpinnerGapIcon, XCircleIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { datastoresQuery } from "@/features/datastores/api"
import { nodesQuery } from "@/features/nodes/api"
import { serversQuery } from "@/features/servers/api"
import { formatAgo, formatBytes } from "@/lib/format"
import { locale } from "@/lib/i18n"
import { cn } from "@/lib/utils"
import { type OverlayPeer, type PortTest, type PublishedPort, useTestOverlayPeer } from "./api"

/** Whether peers had no handshake for 5 minutes, e.g. as a firewall blocks the port, which keepalives renew every 2. */
const stale = (handshake?: string) => !handshake || Date.now() - Date.parse(handshake) > 5 * 60_000

/** The padding of the cells of the peers, less on phones. */
const cell = "px-3 py-2.5 sm:px-4"

const millis = new Intl.NumberFormat(locale, { style: "unit", unit: "millisecond", maximumFractionDigits: 1 })

/** The name of a node, or its ID while the nodes load. */
function useNodeName() {
  const { data: nodes } = useQuery(nodesQuery)
  return (id: string) => nodes?.find((n) => n.id === id)?.name ?? id
}

/** The peers of a member with their handshakes and traffic; those who manage the network test the ports that each publishes for it. */
export function PeerTable({ nodeId, peers, manage }: { nodeId: string; peers: OverlayPeer[]; manage: boolean }) {
  const name = useNodeName()
  return (
    <div className="surface overflow-x-auto rounded-xl">
      <table className="w-full text-sm">
        <thead className="text-left text-xs text-muted-foreground">
          <tr className="border-b">
            <th className={cn(cell, "font-medium")}>{t("Node")}</th>
            <th className={cn(cell, "hidden font-medium sm:table-cell")}>{t("Address")}</th>
            <th className={cn(cell, "hidden font-medium lg:table-cell")}>{t("Endpoint")}</th>
            <th className={cn(cell, "font-medium")}>{t("Latest handshake")}</th>
            <th className={cn(cell, "hidden text-right font-medium md:table-cell")}>{t("Received / sent")}</th>
            {manage && (
              <th className={cell}>
                <span className="sr-only">{t("Test")}</span>
              </th>
            )}
          </tr>
        </thead>
        <tbody className="divide-y">
          {peers.map((p) => (
            <PeerRow key={p.nodeId} nodeId={nodeId} peer={p} name={name(p.nodeId)} manage={manage} />
          ))}
        </tbody>
      </table>
    </div>
  )
}

function PeerRow({ nodeId, peer, name, manage }: { nodeId: string; peer: OverlayPeer; name: string; manage: boolean }) {
  const test = useTestOverlayPeer(nodeId)
  return (
    <>
      <tr>
        <td className={cn(cell, "max-w-32 font-medium sm:max-w-48")}>
          <span className="block truncate">{name}</span>
          <span className="block font-mono text-xs font-normal text-muted-foreground sm:hidden">{peer.address}</span>
        </td>
        <td className={cn(cell, "hidden font-mono text-xs sm:table-cell")}>{peer.address}</td>
        <td className={cn(cell, "hidden font-mono text-xs lg:table-cell")}>{peer.endpoint ?? "–"}</td>
        <td className={cell}>
          {stale(peer.latestHandshake) ? (
            <Pill tone="warning">{peer.latestHandshake ? formatAgo(peer.latestHandshake) : t("never")}</Pill>
          ) : (
            formatAgo(peer.latestHandshake!)
          )}
        </td>
        <td className={cn(cell, "hidden text-right tabular-nums md:table-cell")}>
          {formatBytes(peer.receivedBytes)} / {formatBytes(peer.sentBytes)}
        </td>
        {manage && (
          <td className={cn(cell, "text-right")}>
            <Button
              size="sm"
              variant="outline"
              disabled={test.isPending}
              onClick={() => test.mutate(peer.nodeId)}
              title={t("Connect to the ports that {{name}} publishes for this node", { name })}
              aria-label={t("Test")}
            >
              {test.isPending ? <SpinnerGapIcon className="animate-spin motion-reduce:animate-none" /> : <PlugsConnectedIcon />}
              <span className="hidden sm:inline">{t("Test")}</span>
            </Button>
          </td>
        )}
      </tr>
      {(test.data || test.error) && (
        <tr className="bg-muted/30">
          <td colSpan={6} className={cell} aria-live="polite">
            {test.error ? (
              <p className="text-sm text-destructive">{test.error.message}</p>
            ) : (
              <PortTests peerId={peer.nodeId} peerName={name} tests={test.data!} />
            )}
          </td>
        </tr>
      )}
    </>
  )
}

/** The results of a test, port by port. */
function PortTests({ peerId, peerName, tests }: { peerId: string; peerName: string; tests: PortTest[] }) {
  if (tests.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("{{name}} publishes no port for this node.", { name: peerName })}</p>
  }
  return (
    <ul className="grid gap-1.5 text-sm">
      {tests.map((r, i) => (
        <li key={`${r.port}-${i}`} className="flex flex-wrap items-center gap-x-3 gap-y-0.5">
          {r.error ? (
            <XCircleIcon className="size-4 shrink-0 text-destructive" weight="fill" aria-hidden />
          ) : (
            <CheckCircleIcon className="size-4 shrink-0 text-success" weight="fill" aria-hidden />
          )}
          <span className="sr-only">{r.error ? t("Failed") : t("Reached")}</span>
          <span className="font-mono tabular-nums">{r.port}</span>
          <PortOwner nodeId={peerId} port={r} />
          <span className={r.error ? "text-destructive" : "text-muted-foreground"}>
            {r.error || t("connected in {{time}}", { time: millis.format(r.millis) })}
          </span>
        </li>
      ))}
    </ul>
  )
}

/** The server or datastore of a port on a node, linked; one the user may not see is hidden. */
function PortOwner({ nodeId, port, className }: { nodeId: string; port: PublishedPort | PortTest; className?: string }) {
  const { data: servers } = useQuery({ ...serversQuery(nodeId), enabled: !!port.serverId })
  const { data: datastores } = useQuery({ ...datastoresQuery, enabled: !!port.datastoreId })
  const link = cn("min-w-0 truncate font-medium hover:underline", className)
  if (port.serverId) {
    return (
      <Link to="/nodes/$nodeId/servers/$serverId" params={{ nodeId, serverId: port.serverId }} className={link}>
        {servers?.find((s) => s.id === port.serverId)?.name ?? port.serverId}
      </Link>
    )
  }
  const datastore = datastores?.find((d) => d.id === port.datastoreId)
  if (datastore) {
    return (
      <Link to="/networks/$networkId/databases" params={{ networkId: datastore.networkId }} className={cn(link, "inline-flex items-center gap-1.5")}>
        <DatabaseIcon className="size-4 shrink-0 text-muted-foreground" weight="duotone" />
        <span className="truncate">{datastore.name}</span>
      </Link>
    )
  }
  return <span className={cn("text-muted-foreground", className)}>{port.datastoreId ? t("Datastore") : t("Hidden")}</span>
}

/** The ports a member publishes in the private network, each only for the nodes named with it. */
export function PublishedPorts({ nodeId, published }: { nodeId: string; published: PublishedPort[] }) {
  const name = useNodeName()
  return (
    <div className="space-y-2">
      <div>
        <h3 className="text-sm font-medium">{t("Published ports")}</h3>
        <p className="text-sm text-muted-foreground">{t("Only the nodes named with a port reach it in the private network.")}</p>
      </div>
      {published.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("This node publishes no port in it yet.")}</p>
      ) : (
        <ul className="surface divide-y rounded-xl text-sm">
          {published.map((p) => (
            <li key={p.port} className="flex flex-wrap items-center gap-x-4 gap-y-1.5 px-3 py-2.5 sm:px-4">
              <span className="w-12 font-mono tabular-nums">{p.port}</span>
              <PortOwner nodeId={nodeId} port={p} className="flex-1" />
              <span className="flex flex-wrap gap-1.5">
                {p.clients.map((c) => (
                  <span key={c.address} title={c.stale ? t("It has another key now and reaches the port once the network is applied again.") : c.address}>
                    <Pill tone={c.stale ? "warning" : "neutral"} className={c.nodeId ? undefined : "font-mono"}>
                      {c.nodeId ? name(c.nodeId) : c.address}
                      {c.stale && ` · ${t("old key")}`}
                    </Pill>
                  </span>
                ))}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
