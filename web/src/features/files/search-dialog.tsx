import { MagnifyingGlassIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { useState } from "react"
import { ErrorCallout } from "@/components/callout"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
import { Skeleton } from "@/components/ui/skeleton"
import { useDebounced } from "@/lib/use-debounced"
import { type Match, type ServerFiles, searchQuery } from "./api"

/** Marks where text holds the text searched for, regardless of case. */
function Highlighted({ text, query }: { text: string; query: string }) {
  const parts: (string | { mark: string })[] = []
  const lower = text.toLowerCase()
  const needle = query.toLowerCase()
  let from = 0
  for (let at = lower.indexOf(needle); needle && at >= 0; at = lower.indexOf(needle, from)) {
    parts.push(text.slice(from, at), { mark: text.slice(at, at + needle.length) })
    from = at + needle.length
  }
  parts.push(text.slice(from))
  return parts.map((part, i) =>
    typeof part === "string" ? (
      part
    ) : (
      <mark key={i} className="rounded-sm bg-warning/30 text-foreground">
        {part.mark}
      </mark>
    ),
  )
}

/**
 * Searches the text files of a folder and the folders in it for a text, as the file manager shows them, and opens the
 * file of a match in the editor.
 */
export function SearchDialog({ files, dir, folder }: { files: ServerFiles; dir: string; folder: string }) {
  const [open, setOpen] = useState(false)
  const [text, setText] = useState("")
  const query = useDebounced(text.trim(), 400)
  const { data, isFetching, error } = useQuery({ ...searchQuery(files, dir, query), enabled: open && query !== "" })
  const byFile = new Map<string, Match[]>()
  for (const m of data?.matches ?? []) byFile.set(m.path, [...(byFile.get(m.path) ?? []), m])

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" title={t("Search the files")}>
          <MagnifyingGlassIcon />
          <span className="max-sm:sr-only">{t("Search")}</span>
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("Search the files of {{folder}}", { folder })}</DialogTitle>
          <DialogDescription>
            {t("Finds text in the text files of the folder and the folders in it, regardless of case. Secrets of the server stay hidden.")}
          </DialogDescription>
        </DialogHeader>
        <InputGroup>
          <InputGroupAddon>
            <MagnifyingGlassIcon />
          </InputGroupAddon>
          <InputGroupInput
            type="search"
            autoFocus
            maxLength={200}
            aria-label={t("Text to find")}
            placeholder={t("e.g. a setting or the name of a player")}
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
        </InputGroup>
        <div className="-mx-1 max-h-[55vh] min-h-24 overflow-y-auto px-1" aria-live="polite" aria-busy={isFetching}>
          {!query ? null : error ? (
            <ErrorCallout error={error} />
          ) : !data ? (
            <Skeleton className="h-24 rounded-lg" />
          ) : byFile.size === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
              {t("No text file here holds “{{text}}”.", { text: query })}
            </p>
          ) : (
            <ul className="grid gap-3">
              {[...byFile].map(([path, matches]) => (
                <li key={path} className="overflow-hidden rounded-lg ring-1 ring-foreground/10">
                  <p className="truncate border-b bg-muted/40 px-3 py-1.5 font-mono text-xs font-medium">{path}</p>
                  <ul className="divide-y text-sm">
                    {matches.map((m) => (
                      <li key={m.line}>
                        <Link
                          to="/nodes/$nodeId/servers/$serverId/files"
                          params={{ nodeId: files.nodeId, serverId: files.serverId }}
                          search={{ path: path.split("/").slice(0, -1).join("/") || undefined, edit: path }}
                          onClick={() => setOpen(false)}
                          className="flex min-w-0 gap-3 px-3 py-1.5 hover:bg-muted/60"
                        >
                          <span className="w-10 shrink-0 text-right font-mono text-xs text-muted-foreground tabular-nums">{m.line}</span>
                          <code className="min-w-0 truncate text-xs">
                            <Highlighted text={m.text} query={query} />
                          </code>
                        </Link>
                      </li>
                    ))}
                  </ul>
                </li>
              ))}
            </ul>
          )}
        </div>
        {data && query && (
          <p className="text-xs text-muted-foreground">
            {[
              t("{{count}} matches in {{files}} files searched.", { count: data.matches.length, files: data.files, defaultValue_one: "{{count}} match in {{files}} files searched." }),
              data.truncated && t("The search stopped early. Search a folder further down, or for a longer text."),
              data.tooLarge > 0 && t("{{count}} files larger than 4 MB were left out.", { count: data.tooLarge, defaultValue_one: "{{count}} file larger than 4 MB was left out." }),
            ]
              .filter(Boolean)
              .join(" ")}
          </p>
        )}
      </DialogContent>
    </Dialog>
  )
}
