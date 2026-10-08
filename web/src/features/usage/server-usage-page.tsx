import { ArrowsDownUpIcon, CpuIcon, GaugeIcon, HardDriveIcon, MemoryIcon, UsersIcon } from "@phosphor-icons/react"
import { getRouteApi } from "@tanstack/react-router"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { Meter } from "@/components/meter"
import { StatCard, StatStrip } from "@/components/stat-card"
import { Skeleton } from "@/components/ui/skeleton"
import { type Server, useServer } from "@/features/servers/api"
import { isPaper } from "@/features/servers/server-types"
import { niceBytes } from "@/lib/chart"
import { formatBytes } from "@/lib/format"
import { historyQuery, type ServerUsage, useServerUsage } from "./api"
import { formatCores, formatNumber, formatRate } from "./format"
import { type ChartSpec, UsageHistory } from "./usage-history"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId/usage")

/** Paper and its forks tell their ticks per second, except Folia, which tells those of each region. */
const hasTPS = (type: string) => isPaper(type) && type !== "folia"

/** The Usage tab of a server: what it uses now, and during the last day or week. */
export function ServerUsagePage() {
  const { nodeId, serverId } = route.useParams()
  const { server } = useServer(nodeId, serverId)
  const { usage, isPending, error } = useServerUsage(nodeId, serverId)
  if (!server) return null

  // Scaled to the limits of the server, where it has them.
  const charts: ChartSpec[] = [
    {
      title: t("CPU"),
      series: [{ label: t("CPU"), tone: "series-1", value: (p) => p.cpuMillis }],
      format: formatCores,
      max: server.cpuLimit * 1000,
    },
    {
      title: t("Memory"),
      series: [{ label: t("Memory"), tone: "series-1", value: (p) => p.memoryBytes }],
      format: (v) => formatBytes(Math.round(v)),
      max: usage?.memoryLimitBytes,
      nice: niceBytes,
    },
    { title: t("Players"), series: [{ label: t("Players"), tone: "series-1", value: (p) => p.players }], format: formatNumber },
    ...(hasTPS(server.type)
      ? [
          {
            title: t("Ticks per second"),
            series: [{ label: t("TPS"), tone: "series-1", value: (p) => p.tps }],
            format: formatNumber,
            max: 20,
          } satisfies ChartSpec,
        ]
      : []),
    {
      title: t("Network"),
      series: [
        { label: t("Received"), tone: "series-1", value: (p) => p.networkReceived },
        { label: t("Sent"), tone: "series-2", value: (p) => p.networkSent },
      ],
      format: formatRate,
      nice: niceBytes,
    },
    {
      title: t("Data"),
      // 0 until the agent measured it.
      series: [{ label: t("Data"), tone: "series-1", value: (p) => p.diskBytes || null }],
      format: (v) => formatBytes(Math.round(v)),
      nice: niceBytes,
    },
  ]
  return (
    <>
      {error ? (
        <ErrorCallout error={error} />
      ) : isPending ? (
        <Skeleton className="h-36 rounded-xl" />
      ) : (
        <LiveUsage server={server} usage={usage} />
      )}
      <UsageHistory history={(range) => historyQuery(nodeId, serverId, range)} charts={charts} />
    </>
  )
}

function LiveUsage({ server, usage }: { server: Server; usage?: ServerUsage }) {
  const running = usage?.running
  const idle = t("Not running")
  return (
    <StatStrip className="grid-cols-2 lg:grid-cols-3">
      <StatCard icon={CpuIcon} tone="warning" label={t("CPU")} value={running ? formatCores(usage.cpuMillis) : "–"}>
        {!running ? (
          idle
        ) : server.cpuLimit ? (
          <div className="space-y-2">
            <Meter value={usage.cpuMillis / (server.cpuLimit * 1000)} label={t("CPU used of the limit")} />
            <p>{t("of {{limit}}", { limit: formatCores(server.cpuLimit * 1000) })}</p>
          </div>
        ) : (
          t("Not limited")
        )}
      </StatCard>
      <StatCard icon={MemoryIcon} tone="violet" label={t("Memory")} value={running ? formatBytes(usage.memoryBytes) : "–"}>
        {running && usage.memoryLimitBytes ? (
          <div className="space-y-2">
            <Meter value={usage.memoryBytes / usage.memoryLimitBytes} label={t("Memory used of the limit")} />
            <p>{t("of {{limit}}, including Java's overhead", { limit: formatBytes(usage.memoryLimitBytes) })}</p>
          </div>
        ) : (
          idle
        )}
      </StatCard>
      <StatCard
        icon={UsersIcon}
        tone="success"
        label={t("Players")}
        value={usage?.players ? `${usage.players.online} / ${usage.players.max}` : "–"}
      >
        {!running ? idle : !usage.players ? t("The server doesn't answer yet") : usage.players.names.join(", ") || t("Nobody is online")}
      </StatCard>
      {hasTPS(server.type) && (
        <StatCard icon={GaugeIcon} tone="info" label={t("Ticks per second")} value={usage?.tps ? formatNumber(usage.tps) : "–"}>
          {running ? t("over the last minute, 20 at best") : idle}
        </StatCard>
      )}
      <StatCard icon={ArrowsDownUpIcon} tone="info" label={t("Network")} value={running ? formatRate(usage.networkReceived) : "–"}>
        {running ? t("received, {{rate}} sent", { rate: formatRate(usage.networkSent) }) : idle}
      </StatCard>
      <StatCard icon={HardDriveIcon} tone="neutral" label={t("Data")} value={usage?.diskBytes ? formatBytes(usage.diskBytes) : "–"}>
        {usage?.diskBytes ? t("on the disk, measured every few minutes") : t("Not measured yet")}
      </StatCard>
    </StatStrip>
  )
}
