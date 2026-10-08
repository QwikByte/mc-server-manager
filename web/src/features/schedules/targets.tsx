import { CubeIcon, GraphIcon, HardDrivesIcon, type Icon, TagIcon, XIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { Chip } from "@/components/chip"
import { Button } from "@/components/ui/button"
import { FieldDescription, FieldLegend, FieldSet } from "@/components/ui/field"
import { useAccess } from "@/features/access/use-access"
import { networksQuery } from "@/features/networks/api"
import { nodesQuery } from "@/features/nodes/api"
import { allServersQuery } from "@/features/servers/api"
import { NetworkField, TagField } from "@/features/servers/group-fields"
import { TargetsField } from "@/features/servers/targets-field"
import type { TaskTarget } from "./api"

const key = (target: TaskTarget) =>
  target.kind === "server"
    ? `server:${target.nodeId}/${target.serverId ?? ""}`
    : `${target.kind}:${target.value}:${target.kind === "network" ? (target.role ?? "") : ""}`

/** What the panel knows to name targets and count their servers. */
function useTargetData() {
  const access = useAccess()
  const { data: nodes } = useQuery(nodesQuery)
  const { data: servers } = useQuery(allServersQuery)
  const { data: networks } = useQuery({ ...networksQuery, enabled: access.can("networks.view") })
  return { nodes, servers, networks }
}

/** A target as the panel names it, e.g. "Game servers of Main", and the servers it has now, if known. */
function describe(
  target: TaskTarget,
  { nodes, servers, networks }: ReturnType<typeof useTargetData>,
): { icon: Icon; label: string; count?: number } {
  if (target.kind === "tag")
    return { icon: TagIcon, label: target.value, count: servers?.filter((s) => s.tags.includes(target.value)).length }
  if (target.kind === "network") {
    const n = networks?.find((n) => n.id === target.value)
    if (!n) return { icon: GraphIcon, label: t("A network") }
    const network = n.name
    if (target.role === "proxy") return { icon: GraphIcon, label: t("Proxy of {{network}}", { network }), count: 1 }
    if (target.role === "servers")
      return { icon: GraphIcon, label: t("Game servers of {{network}}", { network }), count: n.backends.length }
    return { icon: GraphIcon, label: t("All servers of {{network}}", { network }), count: n.backends.length + 1 }
  }
  const node = nodes?.find((n) => n.id === target.nodeId)?.name ?? t("Node")
  if (!target.serverId) return { icon: HardDrivesIcon, label: t("{{node}} · all servers", { node }) }
  const server = servers?.find((s) => s.nodeId === target.nodeId && s.id === target.serverId)
  return { icon: CubeIcon, label: server?.name ?? t("Unreachable server") }
}

/** The nodes, servers, tags and networks a task runs on. */
export function TargetChips({ targets }: { targets: TaskTarget[] }) {
  const data = useTargetData()
  if (targets.length === 0) return <p className="text-sm text-muted-foreground">{t("No servers. Edit it to choose some.")}</p>
  return (
    <div className="flex flex-wrap gap-1.5">
      {targets.map((target) => {
        const { icon, label } = describe(target, data)
        return (
          <Chip key={key(target)} icon={icon} className="font-normal">
            {label}
          </Chip>
        )
      })}
    </div>
  )
}

/** Which servers of a network a target names; "all" stands for the empty role, which a select can't have. */
const roles = () => [
  { value: "all" as const, label: t("All servers") },
  { value: "servers" as const, label: t("Game servers") },
  { value: "proxy" as const, label: t("Proxy") },
]

/** Chooses the targets of a task: whole nodes and single servers, and tags and networks, which follow their servers. */
export function TaskTargetsField({ value, onChange }: { value: TaskTarget[]; onChange: (targets: TaskTarget[]) => void }) {
  const data = useTargetData()
  const machines = value.flatMap((target) => (target.kind === "server" ? [{ nodeId: target.nodeId, serverId: target.serverId ?? "" }] : []))
  const groups = value.filter((target) => target.kind !== "server")
  const add = (target: TaskTarget) => !groups.some((other) => key(other) === key(target)) && onChange([...value, target])
  return (
    <>
      <TargetsField
        value={machines}
        onChange={(chosen) =>
          onChange([
            ...chosen.map(
              ({ nodeId, serverId }): TaskTarget => (serverId ? { kind: "server", nodeId, serverId } : { kind: "server", nodeId }),
            ),
            ...groups,
          ])
        }
      />
      <FieldSet>
        <FieldLegend variant="label">{t("Tags and networks")}</FieldLegend>
        <FieldDescription>
          {t(
            "They include the servers that get the tag or join the network later. Whoever may change the tags of a server can put it under this task.",
          )}
        </FieldDescription>
        {groups.length > 0 && (
          <ul className="flex flex-wrap gap-2">
            {groups.map((target) => {
              const { icon: Icon, label, count } = describe(target, data)
              return (
                <li key={key(target)} className="flex items-center gap-2 rounded-lg py-1 pr-1 pl-2.5 text-sm ring-1 ring-border">
                  <Icon className="size-4 shrink-0 text-muted-foreground" />
                  <span className="font-medium">{label}</span>
                  {count !== undefined && (
                    <span className="text-xs text-muted-foreground">
                      {t("{{count}} servers", { count, defaultValue_one: "{{count}} server" })}
                    </span>
                  )}
                  <Button
                    type="button"
                    size="icon-xs"
                    variant="ghost"
                    aria-label={t("Remove {{name}}", { name: label })}
                    onClick={() => onChange(value.filter((other) => key(other) !== key(target)))}
                  >
                    <XIcon />
                  </Button>
                </li>
              )
            })}
          </ul>
        )}
        <div className="grid gap-4 xl:grid-cols-2">
          <TagField tags={[...new Set(data.servers?.flatMap((s) => s.tags))].sort()} onAdd={(tag) => add({ kind: "tag", value: tag })} />
          <NetworkField
            networks={data.networks ?? []}
            roles={roles()}
            onAdd={(network, role) => add({ kind: "network", value: network, ...(role !== "all" && { role }) })}
          />
        </div>
      </FieldSet>
    </>
  )
}
