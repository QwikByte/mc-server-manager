import { HardDrivesIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { Checkbox } from "@/components/ui/checkbox"
import { Skeleton } from "@/components/ui/skeleton"
import { nodesQuery } from "@/features/nodes/api"
import { allServersQuery, type Target } from "./api"

const same = (a: Target, b: Target) => a.nodeId === b.nodeId && a.serverId === b.serverId

/** Chooses servers, e.g. those a task runs on: whole nodes, including servers created later, or single servers. */
export function TargetsField({ value, onChange }: { value: Target[]; onChange: (targets: Target[]) => void }) {
  const { data: nodes, isPending } = useQuery(nodesQuery)
  const { data: servers = [] } = useQuery(allServersQuery)
  const has = (target: Target) => value.some((v) => same(v, target))

  function toggle(target: Target, on: boolean) {
    // A whole node replaces the single servers chosen on it.
    const rest = value.filter((v) => !same(v, target) && !(target.serverId === "" && v.nodeId === target.nodeId))
    onChange(on ? [...rest, target] : rest)
  }

  if (isPending) return <Skeleton className="h-28 rounded-xl" />
  const enrolled = nodes?.filter((n) => n.enrolledAt) ?? []
  if (enrolled.length === 0) return <p className="text-sm text-muted-foreground">{t("Add a node first.")}</p>

  return (
    <ul className="grid gap-3">
      {enrolled.map((node) => {
        const whole = has({ nodeId: node.id, serverId: "" })
        const onNode = servers.filter((s) => s.nodeId === node.id)
        const unknown = value.filter((v) => v.nodeId === node.id && v.serverId && !onNode.some((s) => s.id === v.serverId))
        return (
          <li key={node.id} className="rounded-xl p-4 ring-1 ring-border">
            <label className="flex cursor-pointer items-center gap-3">
              <Checkbox checked={whole} onCheckedChange={(on) => toggle({ nodeId: node.id, serverId: "" }, on === true)} />
              <HardDrivesIcon className="size-4 shrink-0 text-info" />
              {/* The name stays whole; the description moves below it if the line is short. */}
              <span className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-0.5">
                <span className="text-sm font-semibold whitespace-nowrap">{node.name}</span>
                <span className="text-xs text-muted-foreground">{t("All servers, including new ones")}</span>
              </span>
            </label>
            <div className="mt-3 flex flex-wrap gap-2 pl-7">
              {onNode.map((s) => (
                <label
                  key={s.id}
                  className="flex cursor-pointer items-center gap-2 rounded-lg px-2.5 py-1.5 text-sm ring-1 ring-border hover:bg-muted/50 has-disabled:cursor-default has-disabled:opacity-60"
                >
                  <Checkbox
                    checked={whole || has({ nodeId: node.id, serverId: s.id })}
                    disabled={whole}
                    onCheckedChange={(on) => toggle({ nodeId: node.id, serverId: s.id }, on === true)}
                  />
                  {s.name}
                </label>
              ))}
              {unknown.map((target) => (
                <label
                  key={target.serverId}
                  className="flex items-center gap-2 rounded-lg px-2.5 py-1.5 text-sm text-muted-foreground ring-1 ring-border"
                >
                  <Checkbox checked onCheckedChange={() => toggle(target, false)} />
                  {t("Unreachable server")}
                </label>
              ))}
              {node.status !== "online" ? (
                <p className="text-xs text-muted-foreground">{t("The node is offline, so its servers can't be listed.")}</p>
              ) : (
                onNode.length === 0 && <p className="text-xs text-muted-foreground">{t("No servers yet.")}</p>
              )}
            </div>
          </li>
        )
      })}
    </ul>
  )
}
