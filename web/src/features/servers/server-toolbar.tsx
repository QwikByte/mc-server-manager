import { FunnelSimpleIcon, ListIcon, MagnifyingGlassIcon, RowsIcon, SortAscendingIcon, SquaresFourIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { FilterChip } from "@/components/filter-chip"
import { StatusDot } from "@/components/status"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
import { cn } from "@/lib/utils"
import type { NodeServer, ServerState } from "./api"
import {
  type Facts,
  type Grouping,
  groupings,
  type Property,
  properties,
  type ServerSearch,
  type Sort,
  sorts,
  type View,
  valuesOf,
} from "./browse"
import { serverStates, states } from "./server-types"

const propertyLabels: Record<Property, () => string> = {
  network: () => t("Network"),
  node: () => t("Node"),
  type: () => t("Type"),
  tag: () => t("Tag"),
}

/** The values of a property among the servers, with how many servers have each. */
function optionsOf(property: Property, servers: NodeServer[], facts: Facts) {
  const options = new Map<string, { label: string; count: number }>()
  for (const s of servers) {
    for (const [value, label] of valuesOf(property, s, facts)) {
      if (property === "tag" && !value) continue
      options.set(value, { label, count: (options.get(value)?.count ?? 0) + 1 })
    }
  }
  return [...options].map(([value, o]) => ({ value, ...o })).sort((a, b) => a.label.localeCompare(b.label, undefined, { numeric: true }))
}

/**
 * Searches, filters, sorts and groups the servers, and switches between cards and a table. The
 * states show how many servers of the search have each.
 */
export function ServerToolbar({
  servers,
  search,
  onSearch,
  counts,
  total,
  facts,
  hidden,
  view,
}: {
  servers: NodeServer[]
  search: ServerSearch
  onSearch: (change: Partial<ServerSearch>) => void
  counts: Record<ServerState, number>
  total: number
  facts: Facts
  /** Properties the list doesn't offer, e.g. the node on a node's page. */
  hidden: Property[]
  view: View
}) {
  const filters = properties
    .filter((p) => !hidden.includes(p))
    .map((property) => ({ property, options: optionsOf(property, servers, facts) }))
    // Choosing among one type or node filters nothing; a single tag still tells tagged servers apart.
    .filter(({ property, options }) => search[property] || options.length > (property === "tag" ? 0 : 1))
  const active = filters.filter(({ property }) => search[property])
  const filtered = active.length > 0 || search.q || search.state

  return (
    <div className="mb-5 space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <InputGroup className="w-full sm:max-w-xs">
          <InputGroupAddon>
            <MagnifyingGlassIcon />
          </InputGroupAddon>
          <InputGroupInput
            type="search"
            placeholder={t("Search servers")}
            aria-label={t("Search servers")}
            value={search.q ?? ""}
            onChange={(e) => onSearch({ q: e.target.value || undefined })}
          />
        </InputGroup>
        {filters.length > 0 && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline">
                <FunnelSimpleIcon />
                {t("Filter")}
                {active.length > 0 && (
                  <span className="rounded-full bg-primary px-1.5 text-xs text-primary-foreground tabular-nums">{active.length}</span>
                )}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-48">
              {filters.map(({ property, options }) => (
                <DropdownMenuSub key={property}>
                  <DropdownMenuSubTrigger>{propertyLabels[property]()}</DropdownMenuSubTrigger>
                  <DropdownMenuSubContent className="max-h-80 w-56 overflow-y-auto">
                    <DropdownMenuRadioGroup
                      value={search[property] ?? ""}
                      onValueChange={(value) => onSearch({ [property]: value || undefined })}
                    >
                      <DropdownMenuRadioItem value="">{t("All")}</DropdownMenuRadioItem>
                      {options.map((o) => (
                        <DropdownMenuRadioItem key={o.value} value={o.value}>
                          <span className="min-w-0 flex-1 truncate">{o.label}</span>
                          <span className="text-xs text-muted-foreground tabular-nums">{o.count}</span>
                        </DropdownMenuRadioItem>
                      ))}
                    </DropdownMenuRadioGroup>
                  </DropdownMenuSubContent>
                </DropdownMenuSub>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
        <div className="flex flex-wrap items-center gap-2 sm:ml-auto">
          <Choice
            icon={SortAscendingIcon}
            label={t("Sort")}
            value={search.sort ?? "name"}
            options={Object.entries(sorts).map(([value, label]) => ({ value: value as Sort, label: t(label) }))}
            onChange={(sort) => onSearch({ sort: sort === "name" ? undefined : sort })}
          />
          <Choice
            icon={RowsIcon}
            label={t("Group by")}
            value={search.group ?? "none"}
            options={(Object.keys(groupings) as Grouping[])
              .filter((g) => g === "none" || filters.some((f) => f.property === g))
              .map((value) => ({ value, label: t(groupings[value]) }))}
            onChange={(group) => onSearch({ group: group === "none" ? undefined : group })}
          />
          <div role="radiogroup" aria-label={t("Layout")} className="inline-flex rounded-lg bg-muted p-0.5">
            {(
              [
                ["grid", SquaresFourIcon, t("Cards")],
                ["table", ListIcon, t("Table")],
              ] as const
            ).map(([value, Icon, label]) => (
              <button
                key={value}
                type="button"
                role="radio"
                aria-checked={view === value}
                aria-label={label}
                title={label}
                onClick={() => onSearch({ view: value })}
                className="grid h-8 w-9 place-items-center rounded-md text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring aria-checked:bg-card aria-checked:text-foreground aria-checked:shadow-sm"
              >
                <Icon className="size-4" weight="bold" />
              </button>
            ))}
          </div>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <div role="radiogroup" aria-label={t("State")} className="flex max-w-full gap-1 overflow-x-auto rounded-lg bg-muted p-0.5">
          {[undefined, ...states].map((state) => (
            <button
              key={state ?? "all"}
              type="button"
              role="radio"
              aria-checked={search.state === state}
              onClick={() => onSearch({ state })}
              className={cn(
                "flex h-7 items-center gap-2 rounded-md px-2.5 text-xs font-medium whitespace-nowrap text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring aria-checked:bg-card aria-checked:text-foreground aria-checked:shadow-sm",
                state && counts[state] === 0 && search.state !== state && "opacity-50",
              )}
            >
              {state && <StatusDot status={serverStates[state]} />}
              {state ? t(serverStates[state].label) : t("All")}
              <span className="tabular-nums opacity-70">{state ? counts[state] : total}</span>
            </button>
          ))}
        </div>
        {active.map(({ property, options }) => (
          <FilterChip
            key={property}
            label={`${propertyLabels[property]()}: ${options.find((o) => o.value === search[property])?.label ?? search[property]}`}
            onRemove={() => onSearch({ [property]: undefined })}
          />
        ))}
        {filtered && (
          <Button
            variant="ghost"
            size="xs"
            className="text-muted-foreground"
            onClick={() => onSearch({ q: undefined, state: undefined, ...Object.fromEntries(properties.map((p) => [p, undefined])) })}
          >
            {t("Reset filters")}
          </Button>
        )}
      </div>
    </div>
  )
}

/** A dropdown that chooses one of a few options, showing the chosen one. */
function Choice<T extends string>({
  icon: Icon,
  label,
  value,
  options,
  onChange,
}: {
  icon: typeof SortAscendingIcon
  label: string
  value: T
  options: { value: T; label: string }[]
  onChange: (value: T) => void
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" aria-label={`${label}: ${options.find((o) => o.value === value)?.label}`}>
          <Icon />
          <span className="max-sm:hidden">{options.find((o) => o.value === value)?.label}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuLabel>{label}</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={value} onValueChange={(v) => onChange(v as T)}>
          {options.map((o) => (
            <DropdownMenuRadioItem key={o.value} value={o.value}>
              {o.label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
