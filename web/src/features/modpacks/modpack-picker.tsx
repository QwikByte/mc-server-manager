import { DownloadSimpleIcon, MagnifyingGlassIcon } from "@phosphor-icons/react"
import { useInfiniteQuery, useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { useEffect, useState } from "react"
import { ErrorCallout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { Button } from "@/components/ui/button"
import { Field, FieldLabel } from "@/components/ui/field"
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { type SearchHit, searchQuery } from "@/features/plugins/api"
import { PluginIcon } from "@/features/plugins/plugin-icon"
import { locale } from "@/lib/i18n"
import { useDebounced } from "@/lib/use-debounced"
import { describeVersion, type ModpackChoice, modpackVersionsQuery } from "./api"

const compact = new Intl.NumberFormat(locale, { notation: "compact" })

/** Searches Modrinth for a modpack and picks one of its versions, the newest release unless chosen otherwise. */
export function ModpackPicker({ onChange }: { onChange: (choice?: ModpackChoice) => void }) {
  const [input, setInput] = useState("")
  const query = useDebounced(input.trim())
  const [pack, setPack] = useState<SearchHit>()
  const [version, setVersion] = useState<string>()
  const search = useInfiniteQuery({
    ...searchQuery({ query, kind: "modpacks", categories: [], sort: "relevance", serverOnly: false }),
    enabled: !pack,
  })
  const versions = useQuery({ ...modpackVersionsQuery(pack?.id ?? ""), enabled: !!pack })
  const chosen = version ?? versions.data?.find((v) => v.channel === "release")?.id ?? versions.data?.[0]?.id

  useEffect(() => onChange(pack && chosen ? { project: pack.id, version: chosen } : undefined), [pack, chosen, onChange])

  if (pack)
    return (
      <div className="grid grid-cols-1 gap-3 rounded-xl p-3 ring-1 ring-foreground/8">
        <div className="flex items-center gap-3">
          <PluginIcon src={pack.icon} />
          <p className="min-w-0 flex-1 truncate text-sm font-semibold">
            {pack.title}
            <span className="font-normal text-muted-foreground"> {t("by {{author}}", { author: pack.author })}</span>
          </p>
          <Button variant="ghost" size="sm" onClick={() => (setPack(undefined), setVersion(undefined))}>
            {t("Change")}
          </Button>
        </div>
        {versions.error ? (
          <ErrorCallout error={versions.error} />
        ) : (
          <Field>
            <FieldLabel htmlFor="modpack-version">{t("Version")}</FieldLabel>
            {/* Radix reports "" while the options of a new value load; that is no choice. */}
            <Select value={chosen ?? ""} onValueChange={(v) => v && setVersion(v)} disabled={!versions.data?.length}>
              <SelectTrigger id="modpack-version" className="w-full">
                <SelectValue placeholder={versions.isPending ? t("Loading…") : t("No version for servers")} />
              </SelectTrigger>
              <SelectContent>
                {versions.data?.map((v) => (
                  <SelectItem key={v.id} value={v.id}>
                    {describeVersion(v)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
        )}
      </div>
    )

  const hits = search.data?.pages.flatMap((p) => p.hits) ?? []
  return (
    <div className="grid grid-cols-1 gap-2">
      <InputGroup>
        <InputGroupAddon>
          <MagnifyingGlassIcon />
        </InputGroupAddon>
        <InputGroupInput
          type="search"
          placeholder={t("Search modpacks, e.g. Cobblemon")}
          aria-label={t("Search modpacks")}
          value={input}
          onChange={(e) => setInput(e.target.value)}
        />
      </InputGroup>
      {search.isPending ? (
        <Skeleton className="h-40 rounded-xl" />
      ) : search.error ? (
        <ErrorCallout error={search.error} />
      ) : hits.length === 0 ? (
        <p className="py-4 text-center text-sm text-muted-foreground">{t("Nothing found. Try another search.")}</p>
      ) : (
        <ul className="grid max-h-64 grid-cols-1 gap-1 overflow-y-auto rounded-xl p-1 ring-1 ring-foreground/8">
          {hits.map((hit) => (
            <li key={hit.id}>
              <button
                type="button"
                onClick={() => setPack(hit)}
                className="flex w-full items-center gap-3 rounded-lg p-2 text-left transition-colors hover:bg-muted/60 focus-visible:bg-muted/60 focus-visible:outline-none"
              >
                <PluginIcon src={hit.icon} className="size-9" />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium">{hit.title}</span>
                  <span className="block truncate text-xs text-muted-foreground">{hit.description}</span>
                </span>
                <Chip icon={DownloadSimpleIcon}>
                  <span className="sr-only">{t("Downloads")}</span>
                  {compact.format(hit.downloads)}
                </Chip>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
