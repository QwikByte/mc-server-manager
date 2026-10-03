import { MagnifyingGlassIcon, XIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { type ReactNode, useEffect, useRef, useState } from "react"
import { Button } from "@/components/ui/button"
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { nodesQuery } from "@/features/nodes/api"
import { serversQuery } from "@/features/servers/api"
import { categories } from "./meta"
import { type LogSearch, ranges } from "./search"

const all = "all"

/** The filters of the log page in one row, and the narrower ones as removable chips below it. */
export function LogFilters({ search, onChange }: { search: LogSearch; onChange: (change: Partial<LogSearch>) => void }) {
  const [text, setText] = useState(search.search ?? "")
  const typing = useRef(0)
  const { data: nodes = [] } = useQuery(nodesQuery)
  const { data: servers } = useQuery({ ...serversQuery(search.node ?? ""), enabled: !!search.node && !!search.server })
  useEffect(() => () => clearTimeout(typing.current), [])

  // The search applies once typing pauses.
  function type(value: string) {
    setText(value)
    clearTimeout(typing.current)
    typing.current = window.setTimeout(() => onChange({ search: value.trim() || undefined }), 300)
  }

  const node = nodes.find((n) => n.id === search.node)
  const chips: [string, Partial<LogSearch>][] = []
  if (search.hour)
    chips.push([
      t("From {{time}}, one hour", { time: new Date(search.hour).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" }) }),
      { hour: undefined },
    ])
  if (search.user) chips.push([t("User {{user}}", { user: search.user }), { user: undefined }])
  if (search.server)
    chips.push([
      t("Server {{server}}", { server: servers?.find((srv) => srv.id === search.server)?.name ?? search.server }),
      { server: undefined },
    ])
  const filtered = Object.values(search).some(Boolean)

  return (
    <div className="mb-6 space-y-3">
      <div className="flex flex-wrap gap-2">
        <InputGroup className="min-w-56 flex-1">
          <InputGroupAddon>
            <MagnifyingGlassIcon />
          </InputGroupAddon>
          <InputGroupInput
            type="search"
            placeholder={t("Search messages, users, servers, IPs…")}
            aria-label={t("Search the log")}
            value={text}
            onChange={(e) => type(e.target.value)}
          />
        </InputGroup>
        <Choice
          label={t("Time")}
          value={search.hour ? "hour" : search.range}
          onChange={(range) => onChange({ range: range as LogSearch["range"], hour: undefined })}
          everything={t("All time")}
        >
          {search.hour && <SelectItem value="hour">{t("Chosen hour")}</SelectItem>}
          {Object.entries(ranges).map(([value, { label }]) => (
            <SelectItem key={value} value={value}>
              {t(label)}
            </SelectItem>
          ))}
        </Choice>
        <Choice
          label={t("Level")}
          value={search.level}
          onChange={(level) => onChange({ level: level as LogSearch["level"] })}
          everything={t("All levels")}
        >
          <SelectItem value="info">{t("Info and above")}</SelectItem>
          <SelectItem value="warn">{t("Warnings and errors")}</SelectItem>
          <SelectItem value="error">{t("Errors only")}</SelectItem>
        </Choice>
        <Choice
          label={t("Category")}
          value={search.category}
          onChange={(category) => onChange({ category })}
          everything={t("All categories")}
        >
          {Object.entries(categories).map(([value, label]) => (
            <SelectItem key={value} value={value}>
              {t(label)}
            </SelectItem>
          ))}
        </Choice>
        <Choice
          label={t("Source")}
          value={search.source}
          onChange={(source) => onChange({ source: source as LogSearch["source"] })}
          everything={t("Master and agents")}
        >
          <SelectItem value="master">{t("Master")}</SelectItem>
          <SelectItem value="agent">{t("Agents")}</SelectItem>
        </Choice>
        {nodes.length > 0 && (
          <Choice
            label={t("Node")}
            value={search.node}
            onChange={(id) => onChange({ node: id, server: undefined })}
            everything={t("All nodes")}
          >
            {search.node && !node && <SelectItem value={search.node}>{t("Removed node")}</SelectItem>}
            {nodes.map((n) => (
              <SelectItem key={n.id} value={n.id}>
                {n.name}
              </SelectItem>
            ))}
          </Choice>
        )}
      </div>
      {(chips.length > 0 || filtered) && (
        <div className="flex flex-wrap items-center gap-2">
          {chips.map(([label, change]) => (
            <span
              key={label}
              className="inline-flex items-center gap-1 rounded-full bg-primary/10 py-0.5 pr-1 pl-3 text-xs font-medium text-primary"
            >
              {label}
              <button
                type="button"
                aria-label={t("Remove the filter: {{filter}}", { filter: label })}
                onClick={() => onChange(change)}
                className="grid size-5 place-items-center rounded-full outline-none hover:bg-primary/15 focus-visible:ring-2 focus-visible:ring-ring"
              >
                <XIcon className="size-3" weight="bold" />
              </button>
            </span>
          ))}
          {filtered && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                clearTimeout(typing.current)
                setText("")
                onChange(Object.fromEntries(Object.keys(search).map((key) => [key, undefined])))
              }}
            >
              {t("Clear filters")}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}

/** A select whose first option selects everything. */
function Choice({
  label,
  value,
  onChange,
  everything,
  children,
}: {
  label: string
  value?: string
  onChange: (value: string | undefined) => void
  everything: string
  children: ReactNode
}) {
  return (
    <Select value={value ?? all} onValueChange={(v) => onChange(v === all ? undefined : v)}>
      <SelectTrigger aria-label={label} className="max-sm:flex-1">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={all}>{everything}</SelectItem>
        {children}
      </SelectContent>
    </Select>
  )
}
