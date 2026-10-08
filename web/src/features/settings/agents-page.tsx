import { GearIcon, HardDrivesIcon, TerminalWindowIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { IconTile } from "@/components/icon-tile"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useAccess } from "@/features/access/use-access"
import { type Node, nodesQuery } from "@/features/nodes/api"
import { NodeSettingsDialog } from "@/features/nodes/node-settings-dialog"
import { NodeStatusBadge } from "@/features/nodes/node-status"
import { runtimeLabel } from "@/features/nodes/runtime"
import { type OutdatedAgent, updateQuery, waitsForUpdate } from "@/features/updates/api"
import { UpdateAgentButton } from "@/features/updates/update-agent-button"
import { formatDate, formatMegabytes } from "@/lib/format"
import { cn } from "@/lib/utils"

/** The Agents tab: every node with its agent and settings at a glance. */
export function AgentsSettingsPage() {
  const { admin } = useAccess()
  const { data: nodes, isPending, error } = useQuery(nodesQuery)
  // Only administrators update agents.
  const { data: updates } = useQuery({ ...updateQuery, enabled: admin })
  if (isPending) return <Skeleton className="h-64 rounded-xl" />
  if (error) return <ErrorCallout error={error} />
  if (nodes.length === 0) {
    return (
      <EmptyState icon={HardDrivesIcon} tone="info" title={t("No agents yet")}>
        <Button asChild>
          <Link to="/nodes">{t("Go to nodes")}</Link>
        </Button>
      </EmptyState>
    )
  }
  return (
    <div className="surface overflow-hidden rounded-xl">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("Node")}</TableHead>
            <TableHead className="hidden md:table-cell">{t("Agent")}</TableHead>
            <TableHead className="hidden lg:table-cell">{t("Certificate")}</TableHead>
            <TableHead className="hidden sm:table-cell">{t("Ports")}</TableHead>
            <TableHead className="hidden sm:table-cell">{t("Memory")}</TableHead>
            <TableHead className="hidden xl:table-cell">{t("Storage")}</TableHead>
            <TableHead>
              <span className="sr-only">{t("Actions")}</span>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {nodes.map((node) => (
            <AgentRow
              key={node.id}
              node={node}
              outdated={updates?.agents.find((a) => a.nodeId === node.id)}
              version={updates?.version ?? ""}
            />
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

/** A node with its agent; outdated is set while the agent is older than the master. */
function AgentRow({ node, outdated, version }: { node: Node; outdated?: OutdatedAgent; version: string }) {
  const { can } = useAccess()
  return (
    <TableRow>
      <TableCell>
        <span className="flex items-center gap-3">
          <IconTile icon={HardDrivesIcon} tone="info" size="sm" />
          <span className="min-w-0">
            <Link to="/nodes/$nodeId" params={{ nodeId: node.id }} className="block truncate font-medium hover:underline">
              {node.name}
            </Link>
            <span className="block truncate font-mono text-xs text-muted-foreground">{node.address}</span>
          </span>
          <NodeStatusBadge status={node.status} />
        </span>
      </TableCell>
      <TableCell className="hidden md:table-cell">
        {node.info ? (
          <>
            <span className="block font-medium">{node.info.agentVersion}</span>
            {outdated?.update ? (
              <span className={cn("block text-xs", outdated.update.error ? "text-destructive" : "text-muted-foreground")}>
                {outdated.update.error ?? t("updating…")}
              </span>
            ) : (
              <span className="block text-xs text-muted-foreground">{node.info.os || runtimeLabel(node.info)}</span>
            )}
          </>
        ) : (
          <span className="text-muted-foreground">–</span>
        )}
      </TableCell>
      <TableCell className="hidden lg:table-cell">
        {node.certificateExpiresAt ? formatDate(node.certificateExpiresAt) : <span className="text-muted-foreground">–</span>}
      </TableCell>
      <TableCell className="hidden font-mono sm:table-cell">
        {node.portMin === null ? t("any") : `${node.portMin}–${node.portMax}`}
      </TableCell>
      <TableCell className="hidden sm:table-cell">
        {node.memoryReserveMb === null ? t("Not limited") : t("{{memory}} reserved", { memory: formatMegabytes(node.memoryReserveMb) })}
      </TableCell>
      <TableCell className="hidden xl:table-cell">{node.defaultStorage}</TableCell>
      <TableCell>
        <span className="flex items-center justify-end gap-1">
          {outdated && waitsForUpdate(outdated) && <UpdateAgentButton agent={outdated} version={version} />}
          {node.enrolledAt && can("terminal.use") && (
            <Button variant="ghost" size="icon-sm" asChild>
              <Link
                to="/settings/terminal"
                search={{ target: node.id }}
                aria-label={t("Terminal of {{name}}", { name: node.name })}
                title={t("Terminal")}
              >
                <TerminalWindowIcon />
              </Link>
            </Button>
          )}
          {can("nodes.edit", node.id) && (
            <NodeSettingsDialog
              node={node}
              trigger={
                <Button variant="ghost" size="icon-sm" aria-label={t("Settings of {{name}}", { name: node.name })} title={t("Settings")}>
                  <GearIcon />
                </Button>
              }
            />
          )}
        </span>
      </TableCell>
    </TableRow>
  )
}
