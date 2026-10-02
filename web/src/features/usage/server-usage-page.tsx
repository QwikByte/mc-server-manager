import { ArrowsDownUpIcon, CpuIcon, GaugeIcon, HardDriveIcon, MemoryIcon, UsersIcon } from "@phosphor-icons/react"
import { getRouteApi } from "@tanstack/react-router"
import { ErrorCallout } from "@/components/callout"
import { Meter } from "@/components/meter"
import { StatCard } from "@/components/stat-card"
import { Skeleton } from "@/components/ui/skeleton"
import { type Server, useServer } from "@/features/servers/api"
import { niceBytes } from "@/lib/chart"
import { formatBytes } from "@/lib/format"
import { type ServerUsage, useServerUsage } from "./api"
import { formatCores, formatNumber, formatRate } from "./format"
import { type ChartSpec, UsageHistory } from "./usage-history"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId/usage")

/** Only Paper and Purpur tell their ticks per second. */
const hasTPS = (type: string) => type === "paper" || type === "purpur"

/** The Usage tab of a server: what it uses now, and during the last day or week. */
export function ServerUsagePage() {
  const { nodeId, serverId } = route.useParams()
  const { server } = useServer(nodeId, serverId)
  const { usage, isPending, error } = useServerUsage(nodeId, serverId)
  if (!server) return null

  // Scaled to the limits of the server, where it has them.
  const charts: ChartSpec[] = [
    { title: "CPU", series: [{ label: "CPU", tone: "series-1", value: (p) => p.cpuMillis }], format: formatCores, max: server.cpuLimit * 1000 },
    {
      title: "Memory",
      series: [{ label: "Memory", tone: "series-1", value: (p) => p.memoryBytes }],
      format: (v) => formatBytes(Math.round(v)),
      max: usage?.memoryLimitBytes,
      nice: niceBytes,
    },
    { title: "Players", series: [{ label: "Players", tone: "series-1", value: (p) => p.players }], format: formatNumber },
    ...(hasTPS(server.type)
      ? [{ title: "Ticks per second", series: [{ label: "TPS", tone: "series-1", value: (p) => p.tps }], format: formatNumber, max: 20 } satisfies ChartSpec]
      : []),
    {
      title: "Network",
      series: [
        { label: "Received", tone: "series-1", value: (p) => p.networkReceived },
        { label: "Sent", tone: "series-2", value: (p) => p.networkSent },
      ],
      format: formatRate,
      nice: niceBytes,
    },
  ]
  return (
    <>
      {error ? <ErrorCallout error={error} /> : isPending ? <Skeleton className="h-36 rounded-xl" /> : <LiveUsage server={server} usage={usage} />}
      <UsageHistory nodeId={nodeId} serverId={serverId} charts={charts} />
    </>
  )
}

function LiveUsage({ server, usage }: { server: Server; usage?: ServerUsage }) {
  const running = usage?.running
  const idle = "Not running"
  return (
    <div className="grid grid-cols-2 gap-4 lg:grid-cols-3">
      <StatCard icon={CpuIcon} tone="warning" label="CPU" value={running ? formatCores(usage.cpuMillis) : "–"}>
        {!running ? (
          idle
        ) : server.cpuLimit ? (
          <div className="space-y-2">
            <Meter value={usage.cpuMillis / (server.cpuLimit * 1000)} label="CPU used of the limit" />
            <p>of {formatCores(server.cpuLimit * 1000)}</p>
          </div>
        ) : (
          "Not limited"
        )}
      </StatCard>
      <StatCard icon={MemoryIcon} tone="violet" label="Memory" value={running ? formatBytes(usage.memoryBytes) : "–"}>
        {running && usage.memoryLimitBytes ? (
          <div className="space-y-2">
            <Meter value={usage.memoryBytes / usage.memoryLimitBytes} label="Memory used of the limit" />
            <p>of {formatBytes(usage.memoryLimitBytes)}, including what Java needs besides the server</p>
          </div>
        ) : (
          idle
        )}
      </StatCard>
      <StatCard icon={UsersIcon} tone="success" label="Players" value={usage?.players ? `${usage.players.online} / ${usage.players.max}` : "–"}>
        {!running ? idle : !usage.players ? "The server doesn't answer yet" : usage.players.names.join(", ") || "Nobody is online"}
      </StatCard>
      {hasTPS(server.type) && (
        <StatCard icon={GaugeIcon} tone="info" label="Ticks per second" value={usage?.tps ? formatNumber(usage.tps) : "–"}>
          {running ? "over the last minute, 20 at best" : idle}
        </StatCard>
      )}
      <StatCard icon={ArrowsDownUpIcon} tone="info" label="Network" value={running ? formatRate(usage.networkReceived) : "–"}>
        {running ? `received, ${formatRate(usage.networkSent)} sent` : idle}
      </StatCard>
      <StatCard icon={HardDriveIcon} tone="neutral" label="Data" value={usage?.diskBytes ? formatBytes(usage.diskBytes) : "–"}>
        {usage?.diskBytes ? "on the disk, measured every few minutes" : "Not measured yet"}
      </StatCard>
    </div>
  )
}
