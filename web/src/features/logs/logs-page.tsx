import { DownloadSimpleIcon, ScrollIcon } from "@phosphor-icons/react"
import { getRouteApi } from "@tanstack/react-router"
import { useMemo, useState } from "react"
import { PageHeader } from "@/components/page-header"
import { Section } from "@/components/section"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"
import { exportUrl, type LogFilter } from "./api"
import { LogOverview } from "./log-chart"
import { LogFilters } from "./log-filters"
import { LogList } from "./log-list"
import { timeOf } from "./search"

const route = getRouteApi("/_app/logs")

/** The log of the master and its agents, filtered by the address, with new entries streaming in. */
export function LogsPage() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const [live, setLive] = useState(true)
  const { range, hour, ...rest } = search
  const time = useMemo(() => timeOf(range, hour), [range, hour])
  const filter: LogFilter = { ...rest, ...time }
  const update = (change: Partial<typeof search>) => void navigate({ search: (prev) => ({ ...prev, ...change }), replace: true })
  // An hour in the past gets no new entries.
  const streaming = live && !hour

  return (
    <>
      <PageHeader
        icon={ScrollIcon}
        tone="violet"
        title="Logs"
        description="What happened on the master and its agents: who did what, and what went wrong. You see the entries about the nodes and servers your groups allow."
        actions={
          <>
            <Button variant="outline" aria-pressed={live} onClick={() => setLive(!live)} disabled={!!hour}>
              <span aria-hidden className={cn("size-2 rounded-full", streaming ? "animate-pulse bg-success" : "bg-muted-foreground/50")} />
              {streaming ? "Live" : "Paused"}
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline">
                  <DownloadSimpleIcon />
                  Export
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-56">
                <DropdownMenuItem asChild>
                  <a href={exportUrl(filter, "csv")} download>
                    CSV for spreadsheets
                  </a>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <a href={exportUrl(filter, "jsonl")} download>
                    JSON lines for log tools
                  </a>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        }
      />
      <LogFilters search={search} onChange={update} />
      <LogOverview filter={filter} onSelectHour={(start) => update({ hour: start.toISOString(), range: undefined })} />
      <Section title="Entries" description="Newest first. Select an entry for its details." className="mt-8">
        <LogList filter={filter} live={streaming} onFilter={update} />
      </Section>
    </>
  )
}
