import { CaretDownIcon, CaretLeftIcon, CaretRightIcon, CaretUpIcon, DatabaseIcon, FunnelIcon, KeyIcon, TableIcon, XIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi } from "@tanstack/react-router"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { BackLink } from "@/components/back-link"
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { IconTile } from "@/components/icon-tile"
import { usePageName } from "@/components/page-title"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { Table as Grid, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { formatBytes } from "@/lib/format"
import { locale } from "@/lib/i18n"
import { cn } from "@/lib/utils"
import { networkDatastoresQuery, pageQuery, pageRows, type Table, type TableFilter, type TableView, tablesQuery } from "./api"

const route = getRouteApi("/_app/networks/$networkId/databases/$datastoreId/$database")

const count = new Intl.NumberFormat(locale)

/** A table's name, with its schema unless that is PostgreSQL's default. */
const nameOf = (table: Table) => (table.schema && table.schema !== "public" ? `${table.schema}.${table.name}` : table.name)

/** Looks into the tables of a database, which the agent only reads. */
export function TableBrowser() {
  const { networkId, datastoreId, database } = route.useParams()
  usePageName(database)
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const { data: datastores } = useQuery(networkDatastoresQuery(networkId))
  const tables = useQuery(tablesQuery(datastoreId, database))
  const chosen = tables.data?.find((tb) => tb.name === search.table && (tb.schema ?? "") === (search.schema ?? ""))
  const table = chosen ?? tables.data?.[0]
  const view: TableView = chosen
    ? {
        offset: search.offset ?? 0,
        sort: search.sort,
        descending: search.desc,
        filter: search.filter ? { column: search.filter, value: search.value ?? "", contains: !!search.contains } : undefined,
      }
    : { offset: 0 }
  const show = (tb: Table, v: TableView = { offset: 0 }) =>
    navigate({
      search: {
        table: tb.name,
        schema: tb.schema || undefined,
        offset: v.offset || undefined,
        sort: v.sort,
        desc: (v.sort && v.descending) || undefined,
        filter: v.filter?.column,
        value: v.filter?.value,
        contains: v.filter?.contains || undefined,
      },
    })

  return (
    <div>
      <BackLink to="/networks/$networkId/databases" params={{ networkId }}>
        {t("Databases")}
      </BackLink>
      <header className="mb-5 flex items-center gap-3">
        <IconTile icon={DatabaseIcon} tone="violet" size="sm" />
        <div className="min-w-0">
          <h2 className="truncate font-mono text-lg font-semibold">
            {datastores?.find((ds) => ds.id === datastoreId)?.name ?? "…"}
            <span className="text-muted-foreground"> / </span>
            {database}
          </h2>
          <p className="text-sm text-muted-foreground">{t("Read only, with long values cut short.")}</p>
        </div>
      </header>
      {tables.isPending ? (
        <Skeleton className="h-80 rounded-xl" />
      ) : tables.error ? (
        <ErrorCallout error={tables.error} />
      ) : !table ? (
        <EmptyState icon={TableIcon} tone="info" title={t("No tables yet")} description={t("Plugins create their tables when they first connect to the database.")} />
      ) : (
        <div className="grid gap-4 lg:grid-cols-[17rem_minmax(0,1fr)]">
          <nav aria-label={t("Tables")} className="surface max-h-72 overflow-y-auto rounded-xl p-1.5 lg:max-h-[70vh]">
            <ul className="space-y-0.5">
              {tables.data.map((tb) => (
                <li key={`${tb.schema}.${tb.name}`}>
                  <button
                    type="button"
                    aria-current={tb === table || undefined}
                    onClick={() => show(tb)}
                    className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left transition-colors hover:bg-muted aria-[current]:bg-muted aria-[current]:font-semibold"
                  >
                    <TableIcon className="size-4 shrink-0 text-muted-foreground" />
                    <span className="min-w-0 flex-1 truncate font-mono text-xs">{nameOf(tb)}</span>
                    {tb.rows >= 0 && <span className="shrink-0 text-xs text-muted-foreground tabular-nums">≈{count.format(tb.rows)}</span>}
                  </button>
                </li>
              ))}
            </ul>
          </nav>
          <Rows
            key={`${table.schema}.${table.name}`}
            datastoreId={datastoreId}
            database={database}
            table={table}
            view={view}
            onView={(v) => show(table, v)}
          />
        </div>
      )}
    </div>
  )
}

function Rows({
  datastoreId,
  database,
  table,
  view,
  onView,
}: {
  datastoreId: string
  database: string
  table: Table
  view: TableView
  onView: (view: TableView) => void
}) {
  const { data: page, error, isPending, isPlaceholderData } = useQuery(pageQuery(datastoreId, database, table, view))
  const { offset } = view
  // Sorted ascending first, then descending, then in the order of the primary key again.
  const sortBy = (column: string) =>
    onView({
      ...view,
      offset: 0,
      sort: view.sort === column && view.descending ? undefined : column,
      descending: view.sort === column && !view.descending,
    })
  return (
    <section aria-label={nameOf(table)} className="surface min-w-0 overflow-hidden rounded-xl">
      <header className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b px-4 py-2.5">
        <p className="min-w-0 flex-1 truncate font-mono text-sm font-semibold">{nameOf(table)}</p>
        <span className="text-xs text-muted-foreground tabular-nums">
          {page && page.rows.length > 0
            ? t("Rows {{from}}–{{to}} · {{size}}", { from: count.format(offset + 1), to: count.format(offset + page.rows.length), size: formatBytes(table.size) })
            : formatBytes(table.size)}
        </span>
        <div className="flex gap-1">
          <Button
            size="icon-sm"
            variant="outline"
            aria-label={t("Previous rows")}
            disabled={offset === 0 || !page || isPlaceholderData}
            onClick={() => onView({ ...view, offset: Math.max(0, offset - pageRows) })}
          >
            <CaretLeftIcon />
          </Button>
          <Button
            size="icon-sm"
            variant="outline"
            aria-label={t("Next rows")}
            disabled={!page?.more || isPlaceholderData}
            onClick={() => onView({ ...view, offset: offset + pageRows })}
          >
            <CaretRightIcon />
          </Button>
        </div>
      </header>
      <FilterBar
        key={JSON.stringify(view.filter ?? null)}
        columns={page?.columns.map((c) => c.name) ?? []}
        filter={view.filter}
        onFilter={(filter) => onView({ ...view, offset: 0, filter })}
      />
      {isPending ? (
        <Skeleton className="m-4 h-60 rounded-lg" />
      ) : error ? (
        <div className="p-4">
          <ErrorCallout error={error} />
        </div>
      ) : page.rows.length === 0 ? (
        <p className="px-4 py-10 text-center text-sm text-muted-foreground">
          {view.filter ? t("No rows match the filter here.") : t("The table has no rows here.")}
        </p>
      ) : (
        <Grid className={cn("font-mono text-xs transition-opacity", isPlaceholderData && "opacity-60")}>
          <TableHeader>
            <TableRow>
              {page.columns.map((c) => {
                const sorted = view.sort === c.name
                return (
                  <TableHead
                    key={c.name}
                    aria-sort={sorted ? (view.descending ? "descending" : "ascending") : undefined}
                    className="h-auto p-0 tracking-normal normal-case"
                  >
                    <button
                      type="button"
                      onClick={() => sortBy(c.name)}
                      title={t("Sort by {{column}}", { column: c.name })}
                      className="flex w-full flex-col items-start px-3 py-2 text-left outline-none hover:bg-muted/60 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
                    >
                      <span className="flex items-center gap-1 font-semibold text-foreground">
                        {c.primaryKey && <KeyIcon className="size-3 text-amber-500" aria-label={t("Primary key")} />}
                        {c.name}
                        {sorted &&
                          (view.descending ? (
                            <CaretDownIcon className="size-3" aria-label={t("Descending")} />
                          ) : (
                            <CaretUpIcon className="size-3" aria-label={t("Ascending")} />
                          ))}
                      </span>
                      <span className="font-normal">{c.type}</span>
                    </button>
                  </TableHead>
                )
              })}
            </TableRow>
          </TableHeader>
          <TableBody>
            {page.rows.map((row, i) => (
              <TableRow key={offset + i}>
                {row.map((v, j) => (
                  <TableCell key={page.columns[j]?.name ?? j} className="py-1.5">
                    {v.null ? (
                      // i18next-instrument-ignore-next-line: SQL's word for no value
                      <span className="text-muted-foreground italic">NULL</span>
                    ) : (
                      <span className="block max-w-72 truncate" title={v.text}>
                        {v.text}
                        {v.truncated && <span className="text-muted-foreground">…</span>}
                      </span>
                    )}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Grid>
      )}
    </section>
  )
}

/**
 * Filters the rows by the value of a column, as text like the page shows it: those that contain a value, ignoring case,
 * or those that are it exactly.
 */
function FilterBar({ columns, filter, onFilter }: { columns: string[]; filter?: TableFilter; onFilter: (filter?: TableFilter) => void }) {
  const [column, setColumn] = useState(filter?.column ?? "")
  const [contains, setContains] = useState(filter?.contains ?? true)
  const [value, setValue] = useState(filter?.value ?? "")
  const chosen = column || columns[0] || ""

  function submit(event: FormEvent) {
    event.preventDefault()
    if (chosen) onFilter({ column: chosen, value, contains })
  }

  return (
    <form role="search" aria-label={t("Filter the rows")} onSubmit={submit} className="flex flex-wrap items-center gap-2 border-b px-4 py-2.5">
      <FunnelIcon aria-hidden className="size-4 shrink-0 text-muted-foreground max-sm:hidden" />
      <Select value={chosen} onValueChange={setColumn} disabled={columns.length === 0}>
        <SelectTrigger size="sm" aria-label={t("Column")} className="max-w-44 min-w-0 font-mono text-xs">
          <SelectValue placeholder={t("Column")} />
        </SelectTrigger>
        <SelectContent>
          {columns.map((c) => (
            <SelectItem key={c} value={c} className="font-mono text-xs">
              {c}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={contains ? "contains" : "is"} onValueChange={(v) => setContains(v === "contains")}>
        <SelectTrigger size="sm" aria-label={t("Match")} className="text-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="contains">{t("contains")}</SelectItem>
          <SelectItem value="is">{t("is exactly")}</SelectItem>
        </SelectContent>
      </Select>
      <Input
        aria-label={t("Value")}
        placeholder={t("Value")}
        maxLength={1024}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        className="h-7 min-w-32 flex-1 font-mono text-xs"
      />
      <div className="flex gap-1">
        <Button type="submit" size="sm" variant="outline" disabled={!chosen}>
          {t("Filter")}
        </Button>
        {filter && (
          <Button type="button" size="sm" variant="ghost" onClick={() => onFilter(undefined)}>
            <XIcon />
            {t("Clear")}
          </Button>
        )}
      </div>
    </form>
  )
}
