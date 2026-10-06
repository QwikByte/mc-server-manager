import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { Sparkline } from "@/components/sparkline"
import { useAccess } from "@/features/access/use-access"
import { historyQuery } from "@/features/usage/api"
import { cn } from "@/lib/utils"
import type { Node } from "./api"

/** The CPU of a node during the last day as a small line, if the user may see its history. */
export function CpuTrend({ node, caption, className }: { node: Node; caption?: boolean; className?: string }) {
  const { can } = useAccess()
  const { data } = useQuery({ ...historyQuery(node.id, undefined, "day"), enabled: node.status === "online" && can("nodes.view", node.id) })
  if (!data || data.points.length < 2 || !node.info?.cpuCount) return null
  return (
    <div title={t("CPU in the last 24 hours")} className={cn("space-y-1", className)}>
      {caption && <p className="text-xs text-muted-foreground">{t("CPU in the last 24 hours")}</p>}
      <Sparkline values={data.points.map((p) => p.cpuMillis)} max={node.info.cpuCount * 1000} className="text-series-1" />
    </div>
  )
}
