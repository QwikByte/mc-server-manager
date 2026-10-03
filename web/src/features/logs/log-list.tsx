import { CopyIcon, FunnelIcon, ScrollIcon } from "@phosphor-icons/react"
import { useInfiniteQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { useId, useState } from "react"
import { toast } from "sonner"
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { IconTile } from "@/components/icon-tile"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { locale, msg } from "@/lib/i18n"
import { cn } from "@/lib/utils"
import { type LogEntry, type LogFilter, logsQuery, useLiveLogs } from "./api"
import { categoryLabel, formatEntryTime, levels } from "./meta"

/** Labels of attributes that the master and the agents add to entries. */
const attrLabels: Record<string, string> = {
  err: msg("Error"),
  ip: msg("IP address"),
  status: msg("HTTP status"),
  duration: msg("Duration"),
  route: msg("Route"),
  origin: msg("Called by"),
  peer: msg("Master address"),
  code: msg("Error code"),
  command: msg("Command"),
  path: msg("Path"),
  task: msg("Task"),
  account: msg("Account"),
  group: msg("Group"),
}

/** Entries of the log, newest first and grouped by day. Each opens to show its details. */
export function LogList({
  filter,
  live,
  onFilter,
}: {
  filter: LogFilter
  /** Adds new entries as they are logged. */
  live: boolean
  /** Narrows the filter, e.g. to the server of an entry. */
  onFilter?: (change: LogFilter) => void
}) {
  const { data, error, isPending, isPlaceholderData, hasNextPage, fetchNextPage, isFetchingNextPage } = useInfiniteQuery(logsQuery(filter))
  useLiveLogs(filter, live)
  if (isPending) {
    return (
      <div className="grid gap-2">
        {[0, 1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-16 rounded-xl" />
        ))}
      </div>
    )
  }
  if (error) return <ErrorCallout error={error} />
  const entries = data.pages.flat()
  if (entries.length === 0) {
    return (
      <EmptyState
        icon={ScrollIcon}
        tone="neutral"
        title={t("No entries")}
        description={live ? t("Nothing matches yet. New entries show up here as they are logged.") : t("Nothing matches the filter.")}
      />
    )
  }

  // Entries come newest first, so those of a day follow each other.
  const days: LogEntry[][] = []
  for (const entry of entries) {
    const day = days.at(-1)
    if (day && new Date(day[0].time).toDateString() === new Date(entry.time).toDateString()) day.push(entry)
    else days.push([entry])
  }
  return (
    <div className={cn("surface overflow-hidden rounded-xl transition-opacity", isPlaceholderData && "opacity-60")}>
      {days.map((items) => (
        <section key={new Date(items[0].time).toDateString()} aria-label={dayLabel(items[0].time)}>
          <h3 className="border-b bg-muted/50 px-4 py-1.5 text-xs font-semibold text-muted-foreground">{dayLabel(items[0].time)}</h3>
          <ul className="divide-y">
            {items.map((entry) => (
              <LogRow key={entry.id} entry={entry} onFilter={onFilter} />
            ))}
          </ul>
        </section>
      ))}
      {hasNextPage && (
        <div className="border-t p-2 text-center">
          <Button variant="ghost" size="sm" disabled={isFetchingNextPage} onClick={() => void fetchNextPage()}>
            {isFetchingNextPage ? t("Loading…") : t("Show older entries")}
          </Button>
        </div>
      )}
    </div>
  )
}

function dayLabel(iso: string) {
  const day = new Date(iso).toDateString()
  if (day === new Date().toDateString()) return t("Today")
  if (day === new Date(Date.now() - 86_400_000).toDateString()) return t("Yesterday")
  return new Date(iso).toLocaleDateString(locale, { dateStyle: "full" })
}

function LogRow({ entry, onFilter }: { entry: LogEntry; onFilter?: (change: LogFilter) => void }) {
  const [open, setOpen] = useState(false)
  const id = useId()
  const level = levels[entry.level]
  const about = [entry.nodeName ?? entry.nodeId, entry.serverName ?? entry.serverId].filter(Boolean).join(" › ")
  const meta = [categoryLabel(entry.category), entry.user, about, entry.source === "agent" && t("from the agent")].filter(Boolean)

  return (
    <li>
      <button
        type="button"
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen(!open)}
        className="flex w-full items-start gap-3 px-4 py-3 text-left transition-colors outline-none hover:bg-muted/50 focus-visible:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
      >
        <IconTile icon={level.icon} tone={level.tone} size="sm" />
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-medium break-words">
            <span className="sr-only">{t("{{level}}:", { level: t(level.label) })} </span>
            {entry.message}
          </span>
          <span className="mt-0.5 block truncate text-xs text-muted-foreground">
            {meta.join(" · ")}
            {entry.attrs.err && (
              <span className={cn(entry.level === "error" ? "text-destructive" : "text-warning")}> · {entry.attrs.err}</span>
            )}
          </span>
        </span>
        <time dateTime={entry.time} className="shrink-0 pt-0.5 text-xs text-muted-foreground tabular-nums">
          {formatEntryTime(entry.time)}
        </time>
      </button>
      {open && <EntryDetails id={id} entry={entry} onFilter={onFilter} />}
    </li>
  )
}

function EntryDetails({ id, entry, onFilter }: { id: string; entry: LogEntry; onFilter?: (change: LogFilter) => void }) {
  const facts: [string, string | undefined][] = [
    [t("Time"), new Date(entry.time).toLocaleString(locale, { dateStyle: "full", timeStyle: "medium" })],
    [t("Level"), t(levels[entry.level].label)],
    [t("Logged by"), entry.source === "agent" ? t("The agent of {{node}}", { node: entry.nodeName ?? entry.nodeId }) : t("The master")],
    [t("Category"), categoryLabel(entry.category)],
    [t("User"), entry.user],
    [t("Node"), entry.nodeId && `${entry.nodeName ?? t("Removed node")} (${entry.nodeId})`],
    [t("Server"), entry.serverId && `${entry.serverName ?? t("Unknown server")} (${entry.serverId})`],
    ...Object.entries(entry.attrs)
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([key, value]): [string, string] => [attrLabels[key] ? t(attrLabels[key]) : key, value]),
  ]
  const narrow: [string, LogFilter][] = []
  if (entry.user) narrow.push([t("Actions of {{user}}", { user: entry.user }), { user: entry.user }])
  if (entry.serverId) narrow.push([t("This server"), { node: entry.nodeId, server: entry.serverId }])
  else if (entry.nodeId) narrow.push([t("This node"), { node: entry.nodeId }])
  narrow.push([categoryLabel(entry.category), { category: entry.category }])

  return (
    <div id={id} className="border-t border-dashed bg-muted/30 px-4 py-4 sm:pl-15">
      <dl className="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[minmax(8rem,auto)_1fr]">
        {facts
          .filter(([, value]) => value)
          .map(([term, value]) => (
            <div key={term} className="contents">
              <dt className="text-xs text-muted-foreground sm:pt-0.5">{term}</dt>
              <dd className="min-w-0 font-mono text-xs break-all max-sm:mb-1 sm:pt-0.5">{value}</dd>
            </div>
          ))}
      </dl>
      <div className="mt-4 flex flex-wrap gap-2">
        {onFilter &&
          narrow.map(([label, change]) => (
            <Button key={label} variant="outline" size="sm" onClick={() => onFilter(change)}>
              <FunnelIcon />
              {label}
            </Button>
          ))}
        <Button
          variant="ghost"
          size="sm"
          onClick={() =>
            navigator.clipboard.writeText(JSON.stringify(entry, null, 2)).then(
              () => toast.success(t("Copied the entry")),
              () => toast.error(t("The entry can't be copied here.")),
            )
          }
        >
          <CopyIcon />
          {t("Copy as JSON")}
        </Button>
      </div>
    </div>
  )
}
