import { CaretLeftIcon, CaretRightIcon, DatabaseIcon, KeyIcon, TableIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi } from "@tanstack/react-router"
import { t } from "i18next"
import { BackLink } from "@/components/back-link"
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { IconTile } from "@/components/icon-tile"
import { usePageName } from "@/components/page-title"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Table as Grid, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { formatBytes } from "@/lib/format"
import { locale } from "@/lib/i18n"
import { cn } from "@/lib/utils"
import { networkDatastoresQuery, pageQuery, pageRows, type Table, tablesQuery } from "./api"

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
  const show = (tb: Table, offset = 0) => navigate({ search: { table: tb.name, schema: tb.schema || undefined, offset: offset || undefined } })

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
          <Rows key={`${table.schema}.${table.name}`} datastoreId={datastoreId} database={database} table={table} offset={chosen ? (search.offset ?? 0) : 0} onPage={(offset) => show(table, offset)} />
        </div>
      )}
    </div>
  )
}

function Rows({
  datastoreId,
  database,
  table,
  offset,
  onPage,
}: {
  datastoreId: string
  database: string
  table: Table
  offset: number
  onPage: (offset: number) => void
}) {
  const { data: page, error, isPending, isPlaceholderData } = useQuery(pageQuery(datastoreId, database, table, offset))
  if (isPending) return <Skeleton className="h-80 rounded-xl" />
  if (error) return <ErrorCallout error={error} />
  return (
    <section aria-label={nameOf(table)} className="surface min-w-0 overflow-hidden rounded-xl">
      <header className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b px-4 py-2.5">
        <p className="min-w-0 flex-1 truncate font-mono text-sm font-semibold">{nameOf(table)}</p>
        <span className="text-xs text-muted-foreground tabular-nums">
          {page.rows.length > 0
            ? t("Rows {{from}}–{{to}} · {{size}}", { from: count.format(offset + 1), to: count.format(offset + page.rows.length), size: formatBytes(table.size) })
            : formatBytes(table.size)}
        </span>
        <div className="flex gap-1">
          <Button size="icon-sm" variant="outline" aria-label={t("Previous rows")} disabled={offset === 0 || isPlaceholderData} onClick={() => onPage(Math.max(0, offset - pageRows))}>
            <CaretLeftIcon />
          </Button>
          <Button size="icon-sm" variant="outline" aria-label={t("Next rows")} disabled={!page.more || isPlaceholderData} onClick={() => onPage(offset + pageRows)}>
            <CaretRightIcon />
          </Button>
        </div>
      </header>
      {page.rows.length === 0 ? (
        <p className="px-4 py-10 text-center text-sm text-muted-foreground">{t("The table has no rows here.")}</p>
      ) : (
        <Grid className={cn("font-mono text-xs transition-opacity", isPlaceholderData && "opacity-60")}>
          <TableHeader>
            <TableRow>
              {page.columns.map((c) => (
                <TableHead key={c.name} className="h-auto py-2 tracking-normal normal-case">
                  <span className="flex items-center gap-1 font-semibold text-foreground">
                    {c.primaryKey && <KeyIcon className="size-3 text-amber-500" aria-label={t("Primary key")} />}
                    {c.name}
                  </span>
                  <span className="font-normal">{c.type}</span>
                </TableHead>
              ))}
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
