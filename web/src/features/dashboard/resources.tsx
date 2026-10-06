import { ChartLineIcon } from "@phosphor-icons/react"
import { useQueries, useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { useState } from "react"
import { Segmented } from "@/components/segmented"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { nodesQuery } from "@/features/nodes/api"
import { historyQuery, type UsagePoint } from "@/features/usage/api"
import { formatCores } from "@/features/usage/format"
import { TimeChart } from "@/features/usage/time-chart"
import type { ChartSpec } from "@/features/usage/usage-history"
import { niceBytes } from "@/lib/chart"
import { formatBytes } from "@/lib/format"
import { Calm, Panel } from "./panel"

type Measure = "cpu" | "memory"

/** What all nodes the user may see used together during the last day, one measure at a time. */
export function Resources({ title }: { title: string }) {
  const { can } = useAccess()
  const [measure, setMeasure] = useState<Measure>("cpu")
  const { data: nodes = [] } = useQuery(nodesQuery)
  const shown = nodes.filter((n) => n.info && can("nodes.view", n.id))
  const histories = useQueries({ queries: shown.map((n) => historyQuery(n.id, undefined, "day")) })
  const loaded = histories.flatMap((h) => h.data ?? [])
  // The steps of all nodes start at the same times, so their values add up.
  const sums = new Map<string, UsagePoint>()
  for (const p of loaded.flatMap((h) => h.points)) {
    const sum = sums.get(p.time) ?? { ...p, cpuMillis: 0, memoryBytes: 0 }
    sums.set(p.time, { ...sum, cpuMillis: sum.cpuMillis + p.cpuMillis, memoryBytes: sum.memoryBytes + p.memoryBytes })
  }
  const points = [...sums.values()].sort((a, b) => Date.parse(a.time) - Date.parse(b.time))
  const total = (f: (info: NonNullable<(typeof shown)[number]["info"]>) => number) => shown.reduce((sum, n) => sum + (n.info ? f(n.info) : 0), 0)
  const chart: ChartSpec =
    measure === "cpu"
      ? {
          title: t("CPU of all nodes, last 24 hours"),
          series: [{ label: t("CPU"), tone: "series-1", value: (p) => p.cpuMillis }],
          format: formatCores,
          max: total((i) => i.cpuCount * 1000),
        }
      : {
          title: t("Memory of all nodes, last 24 hours"),
          series: [{ label: t("Memory"), tone: "series-2", value: (p) => p.memoryBytes }],
          format: (v) => formatBytes(Math.round(v)),
          max: total((i) => i.memoryBytes),
          nice: niceBytes,
        }

  return (
    <Panel
      title={title}
      actions={
        <Segmented
          label={t("Measure")}
          value={measure}
          onChange={setMeasure}
          options={[
            { value: "cpu", label: t("CPU") },
            { value: "memory", label: t("Memory") },
          ]}
        />
      }
    >
      {shown.length === 0 ? (
        <Calm icon={ChartLineIcon} tone="info">
          {t("No node tells what it uses yet.")}
        </Calm>
      ) : loaded.length === 0 ? (
        <Skeleton className="m-4 h-48 rounded-xl" />
      ) : (
        <TimeChart
          {...chart}
          points={points}
          step={loaded[0]?.step ?? 300}
          span={24 * 3_600_000}
          end={Math.max(...histories.map((h) => h.dataUpdatedAt))}
          className="rounded-none shadow-none ring-0"
        />
      )}
    </Panel>
  )
}
