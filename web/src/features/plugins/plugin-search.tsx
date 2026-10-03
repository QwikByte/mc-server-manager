import { DownloadSimpleIcon, MagnifyingGlassIcon } from "@phosphor-icons/react"
import { useInfiniteQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { type ReactNode, useState } from "react"
import { ErrorCallout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { Button } from "@/components/ui/button"
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
import { Skeleton } from "@/components/ui/skeleton"
import { useDebounced } from "@/lib/use-debounced"
import { type SearchHit, searchQuery } from "./api"
import { PluginIcon } from "./plugin-icon"
import { locale } from "@/lib/i18n"

const downloads = new Intl.NumberFormat(locale, { notation: "compact" })

/**
 * Searches Modrinth for plugins and mods of a server type and Minecraft version. Without
 * a query, the most downloaded ones are shown.
 */
export function PluginSearch({
  type,
  version,
  action,
  autoFocus,
}: {
  type?: string
  version?: string
  action: (hit: SearchHit) => ReactNode
  autoFocus?: boolean
}) {
  const [input, setInput] = useState("")
  const query = useDebounced(input.trim())
  const { data, error, isPending, fetchNextPage, hasNextPage, isFetchingNextPage } = useInfiniteQuery(searchQuery({ query, type, version }))
  const hits = data?.pages.flatMap((p) => p.hits) ?? []

  return (
    <div className="grid grid-cols-1 gap-4">
      <InputGroup>
        <InputGroupAddon>
          <MagnifyingGlassIcon />
        </InputGroupAddon>
        <InputGroupInput
          type="search"
          placeholder={t("Search Modrinth, e.g. LuckPerms")}
          aria-label={t("Search plugins and mods")}
          autoFocus={autoFocus}
          value={input}
          onChange={(e) => setInput(e.target.value)}
        />
      </InputGroup>
      {isPending ? (
        <div className="grid gap-2">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-20 rounded-xl" />
          ))}
        </div>
      ) : error ? (
        <ErrorCallout error={error} />
      ) : hits.length === 0 ? (
        <p className="py-6 text-center text-sm text-muted-foreground">{t("Nothing found. Try another search.")}</p>
      ) : (
        <ul className="grid grid-cols-1 gap-2">
          {hits.map((hit) => (
            <li key={hit.id} className="flex items-start gap-3 rounded-xl p-3 ring-1 ring-foreground/8 transition-colors hover:bg-muted/50">
              <PluginIcon src={hit.icon} />
              <div className="min-w-0 flex-1 space-y-1">
                <p className="truncate text-sm font-semibold">
                  <a href={`https://modrinth.com/project/${hit.slug}`} target="_blank" rel="noreferrer" className="hover:underline">
                    {hit.title}
                  </a>
                  <span className="font-normal text-muted-foreground"> {t("by {{author}}", { author: hit.author })}</span>
                </p>
                <p className="line-clamp-2 text-xs text-muted-foreground">{hit.description}</p>
                <div className="flex flex-wrap gap-1.5 pt-1">
                  <Chip icon={DownloadSimpleIcon}>{downloads.format(hit.downloads)}</Chip>
                  {/* With a type, all results suit it; otherwise they tell what they run on. */}
                  {!type &&
                    hit.loaders.slice(0, 5).map((l) => (
                      <Chip key={l} className="font-normal capitalize">
                        {l}
                      </Chip>
                    ))}
                </div>
              </div>
              <div className="shrink-0 self-center">{action(hit)}</div>
            </li>
          ))}
        </ul>
      )}
      {hasNextPage && (
        <Button variant="outline" className="justify-self-center" disabled={isFetchingNextPage} onClick={() => fetchNextPage()}>
          {isFetchingNextPage ? t("Loading…") : t("Show more")}
        </Button>
      )}
    </div>
  )
}
