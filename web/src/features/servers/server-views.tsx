import { CaretRightIcon, CpuIcon, GraphIcon, HardDrivesIcon, HashIcon, MemoryIcon, UsersIcon } from "@phosphor-icons/react"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { motion } from "motion/react"
import type { ReactNode } from "react"
import { Chip } from "@/components/chip"
import { IconTile } from "@/components/icon-tile"
import { Checkbox } from "@/components/ui/checkbox"
import { SortableHead, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { PinButton } from "@/features/preferences/pin-button"
import { formatCores } from "@/features/usage/format"
import { formatBytes, formatMegabytes } from "@/lib/format"
import { rise } from "@/lib/motion"
import type { Sorting } from "@/lib/sort"
import { cn } from "@/lib/utils"
import { type NodeServer, serverKey } from "./api"
import type { Facts, Group, Sort } from "./browse"
import { ServerActions } from "./server-actions"
import { ServerStateBadge } from "./server-state"
import { displayVersion, memoryTitle, serverLook, serverType } from "./server-types"
import { TagList } from "./tags"

/** How the servers of a list are shown and selected. */
export interface ViewProps {
  groups: Group[]
  facts: Facts
  sorting: Sorting<Sort>
  /** Whether servers of several nodes are listed, which then show their node. */
  showNode: boolean
  selected: (s: NodeServer) => boolean
  onSelect: (servers: NodeServer[], selected: boolean) => void
  collapsed: (group: Group) => boolean
  onCollapse: (group: Group) => void
}

const live = (facts: Facts, s: NodeServer) => {
  const usage = facts.usage(s)
  return usage?.running ? usage : undefined
}

function ServerLink({ server, className }: { server: NodeServer; className?: string }) {
  return (
    <Link to="/nodes/$nodeId/servers/$serverId" params={{ nodeId: server.nodeId, serverId: server.id }} className={className}>
      {server.name}
    </Link>
  )
}

/** Selects all servers of a group, or some of them. */
function SelectAll({
  servers,
  selected,
  onSelect,
  label,
}: Pick<ViewProps, "selected" | "onSelect"> & { servers: NodeServer[]; label: string }) {
  const count = servers.filter(selected).length
  return (
    <Checkbox
      aria-label={label}
      checked={count > 0 && count === servers.length ? true : count > 0 ? "indeterminate" : false}
      onCheckedChange={(on) => onSelect(servers, on === true)}
    />
  )
}

/** The heading of a group, which folds it away. */
function GroupHeading({
  group,
  facts,
  collapsed,
  onCollapse,
  children,
}: {
  group: Group
  facts: Facts
  collapsed: boolean
  onCollapse: () => void
  children: ReactNode
}) {
  const running = group.servers.filter((s) => s.state !== "stopped").length
  const players = group.servers.reduce((sum, s) => sum + (live(facts, s)?.players?.online ?? 0), 0)
  return (
    <div className="flex items-center gap-3">
      {children}
      <button
        type="button"
        aria-expanded={!collapsed}
        onClick={onCollapse}
        className="flex min-w-0 flex-1 items-center gap-2 rounded-md text-left outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <CaretRightIcon
          className={cn("size-3.5 shrink-0 text-muted-foreground transition-transform", !collapsed && "rotate-90")}
          weight="bold"
        />
        <span className="truncate text-sm font-semibold">{group.label}</span>
        <span className="text-xs whitespace-nowrap text-muted-foreground">
          {t("{{running}} of {{count}} running", { running, count: group.servers.length })}
          {players > 0 && ` · ${t("{{count}} players", { count: players, defaultValue_one: "{{count}} player" })}`}
        </span>
      </button>
    </div>
  )
}

/** Cards of servers, each opening its server, in groups that fold away. */
export function ServerGrid({ groups, facts, showNode, selected, onSelect, collapsed, onCollapse }: ViewProps) {
  return (
    <div className="space-y-8">
      {groups.map((group) => (
        <section key={group.key} aria-label={group.label || undefined}>
          {!group.label && (
            <label className="mb-3 inline-flex items-center gap-2 text-sm text-muted-foreground">
              <SelectAll servers={group.servers} selected={selected} onSelect={onSelect} label={t("Select all servers")} />
              {t("Select all")}
            </label>
          )}
          {group.label && (
            <div className="mb-3">
              <GroupHeading group={group} facts={facts} collapsed={collapsed(group)} onCollapse={() => onCollapse(group)}>
                <SelectAll
                  servers={group.servers}
                  selected={selected}
                  onSelect={onSelect}
                  label={t("Select the servers of {{group}}", { group: group.label })}
                />
              </GroupHeading>
            </div>
          )}
          {!collapsed(group) && (
            <ul className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
              {group.servers.map((server, i) => (
                <ServerCard
                  key={serverKey(server)}
                  index={i}
                  server={server}
                  facts={facts}
                  showNode={showNode}
                  selected={selected(server)}
                  onSelect={(on) => onSelect([server], on)}
                />
              ))}
            </ul>
          )}
        </section>
      ))}
    </div>
  )
}

/**
 * The whole card opens the server; its buttons sit above the link. Running servers show what they
 * use.
 */
function ServerCard({
  index,
  server,
  facts,
  showNode,
  selected,
  onSelect,
}: {
  index: number
  server: NodeServer
  facts: Facts
  showNode: boolean
  selected: boolean
  onSelect: (selected: boolean) => void
}) {
  const usage = live(facts, server)
  const network = facts.network(server)
  const look = serverLook(server.type)
  return (
    <motion.li
      {...rise(index)}
      className={cn(
        "group surface lift relative flex flex-col gap-4 rounded-xl p-5 hover:ring-primary/30",
        selected && "ring-2 ring-primary/50 hover:ring-primary/50",
      )}
    >
      <div className="flex items-start gap-3">
        <IconTile icon={look.icon} tone={look.tone} />
        <div className="min-w-0 flex-1">
          <ServerLink
            server={server}
            className="block truncate font-semibold outline-none after:absolute after:inset-0 after:rounded-xl focus-visible:after:ring-2 focus-visible:after:ring-ring"
          />
          <p className="truncate text-xs text-muted-foreground">
            {serverType(server.type).label} {displayVersion(server.version)}
          </p>
        </div>
        <ServerStateBadge server={server} nodeId={server.nodeId} />
        {/* Shows on hover, and always once pinned or without a pointer that hovers. */}
        <PinButton
          nodeId={server.nodeId}
          server={server}
          size="icon-xs"
          className="relative z-10 -my-0.5 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 aria-pressed:opacity-100 pointer-coarse:opacity-100"
        />
        <Checkbox
          checked={selected}
          aria-label={t("Select {{name}}", { name: server.name })}
          onCheckedChange={(on) => onSelect(on === true)}
          className="relative z-10 mt-1"
        />
      </div>
      <div className="flex flex-wrap gap-2">
        {network && <Chip icon={GraphIcon}>{network.name}</Chip>}
        {showNode && <Chip icon={HardDrivesIcon}>{server.nodeName}</Chip>}
        <Chip icon={HashIcon}>
          <span className="font-mono">{server.port}</span>
        </Chip>
        {/* While it runs, its memory of the container's limit, which includes what Java needs besides the heap. */}
        <Chip icon={MemoryIcon} title={memoryTitle(server)}>
          {usage?.memoryLimitBytes
            ? `${formatBytes(usage.memoryBytes)} / ${formatBytes(usage.memoryLimitBytes)}`
            : formatMegabytes(server.memoryMb)}
        </Chip>
        {usage && <Chip icon={CpuIcon}>{formatCores(usage.cpuMillis)}</Chip>}
        {usage?.players && (
          <Chip icon={UsersIcon}>
            {usage.players.online} / {usage.players.max}
          </Chip>
        )}
      </div>
      <TagList tags={server.tags} />
      <div className="relative z-10 mt-auto border-t pt-4 empty:hidden">
        <ServerActions nodeId={server.nodeId} server={server} />
      </div>
    </motion.li>
  )
}

/** A compact table of servers, for many of them, in groups that fold away. */
export function ServerTable({ groups, facts, sorting, showNode, selected, onSelect, collapsed, onCollapse }: ViewProps) {
  const all = groups.flatMap((g) => g.servers)
  const showNetwork = all.some((s) => facts.network(s))
  const columns = 7 + Number(showNode) + Number(showNetwork)
  return (
    <div className="surface overflow-hidden rounded-xl">
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="w-10 pl-4">
              <SelectAll servers={[...new Set(all)]} selected={selected} onSelect={onSelect} label={t("Select all servers")} />
            </TableHead>
            <SortableHead sorting={sorting} column="name">{t("Server")}</SortableHead>
            <SortableHead sorting={sorting} column="state">{t("State")}</SortableHead>
            {showNode && <SortableHead sorting={sorting} column="node" className="max-md:hidden">{t("Node")}</SortableHead>}
            {showNetwork && <TableHead className="max-lg:hidden">{t("Network")}</TableHead>}
            <SortableHead sorting={sorting} column="players" className="text-right">{t("Players")}</SortableHead>
            <SortableHead sorting={sorting} column="cpu" className="text-right max-sm:hidden">{t("CPU")}</SortableHead>
            <SortableHead sorting={sorting} column="memory" className="text-right max-sm:hidden">{t("Memory")}</SortableHead>
            <TableHead className="w-0">
              <span className="sr-only">{t("Actions")}</span>
            </TableHead>
          </TableRow>
        </TableHeader>
        {groups.map((group) => (
          <TableBody key={group.key}>
            {group.label && (
              <TableRow className="bg-muted/40 hover:bg-muted/40">
                <TableCell colSpan={columns} className="py-2 pl-4">
                  <GroupHeading group={group} facts={facts} collapsed={collapsed(group)} onCollapse={() => onCollapse(group)}>
                    <SelectAll
                      servers={group.servers}
                      selected={selected}
                      onSelect={onSelect}
                      label={t("Select the servers of {{group}}", { group: group.label })}
                    />
                  </GroupHeading>
                </TableCell>
              </TableRow>
            )}
            {!collapsed(group) &&
              group.servers.map((server) => {
                const usage = live(facts, server)
                const look = serverLook(server.type)
                return (
                  <TableRow key={serverKey(server)} data-state={selected(server) ? "selected" : undefined}>
                    <TableCell className="pl-4">
                      <Checkbox
                        checked={selected(server)}
                        aria-label={t("Select {{name}}", { name: server.name })}
                        onCheckedChange={(on) => onSelect([server], on === true)}
                      />
                    </TableCell>
                    <TableCell className="max-w-72 min-w-48 whitespace-normal">
                      <div className="flex items-center gap-3">
                        <IconTile icon={look.icon} tone={look.tone} size="sm" />
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                            <ServerLink server={server} className="truncate font-semibold hover:underline" />
                            <TagList tags={server.tags} />
                          </div>
                          <p className="truncate text-xs text-muted-foreground">
                            {serverType(server.type).label} {displayVersion(server.version)} ·{" "}
                            <span className="font-mono">{server.port}</span>
                          </p>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <ServerStateBadge server={server} nodeId={server.nodeId} />
                    </TableCell>
                    {showNode && <TableCell className="text-muted-foreground max-md:hidden">{server.nodeName}</TableCell>}
                    {showNetwork && (
                      <TableCell className="text-muted-foreground max-lg:hidden">{facts.network(server)?.name ?? "–"}</TableCell>
                    )}
                    <TableCell className="text-right tabular-nums">
                      {usage?.players ? `${usage.players.online} / ${usage.players.max}` : <span className="text-muted-foreground">–</span>}
                    </TableCell>
                    <TableCell className="text-right tabular-nums max-sm:hidden">
                      {usage ? formatCores(usage.cpuMillis) : <span className="text-muted-foreground">–</span>}
                    </TableCell>
                    <TableCell className="text-right tabular-nums max-sm:hidden">
                      {usage ? (
                        formatBytes(usage.memoryBytes)
                      ) : (
                        <span className="text-muted-foreground">{formatMegabytes(server.memoryMb)}</span>
                      )}
                    </TableCell>
                    <TableCell className="pr-3">
                      <ServerActions nodeId={server.nodeId} server={server} compact pin />
                    </TableCell>
                  </TableRow>
                )
              })}
          </TableBody>
        ))}
      </Table>
    </div>
  )
}
