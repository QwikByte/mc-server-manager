import { PlusIcon, TagIcon, XIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { type FormEvent, useRef, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { FieldError } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"
import { allServersQuery, type Server, useChangeTags } from "./api"

type TaggedServer = Pick<Server, "id" | "name" | "tags"> & { nodeId: string }

const maxTags = 10
const valid = /^[\p{L}\p{N}_-]{1,24}$/u

/** A tag keeps its colour everywhere, derived from its name. */
function hue(tag: string) {
  let h = 0
  for (const c of tag) h = (h * 31 + c.codePointAt(0)!) % 360
  return h
}

/** A tag as a small label with its colour; with onRemove, it has a button to remove it. */
export function TagChip({ tag, detail, onRemove, className }: { tag: string; detail?: string; onRemove?: () => void; className?: string }) {
  return (
    <span
      className={cn(
        "inline-flex h-5 items-center gap-1 rounded-md bg-muted px-1.5 text-[0.6875rem] font-medium whitespace-nowrap",
        className,
      )}
    >
      <span aria-hidden className="size-1.5 rounded-full" style={{ backgroundColor: `hsl(${hue(tag)} 70% 50%)` }} />
      {tag}
      {detail && <span className="text-muted-foreground">{detail}</span>}
      {onRemove && (
        <button
          type="button"
          aria-label={t("Remove the tag {{tag}}", { tag })}
          onClick={onRemove}
          className="-mr-1 grid size-4 place-items-center rounded outline-none hover:bg-foreground/10 focus-visible:ring-2 focus-visible:ring-ring"
        >
          <XIcon className="size-2.5" weight="bold" />
        </button>
      )}
    </span>
  )
}

export function TagList({ tags, className }: { tags: string[]; className?: string }) {
  if (tags.length === 0) return null
  return (
    <span className={cn("flex flex-wrap gap-1", className)}>
      {tags.map((tag) => (
        <TagChip key={tag} tag={tag} />
      ))}
    </span>
  )
}

/**
 * Adds and removes tags of one or many servers. With many, a tag only some of them have shows
 * how many; removing it removes it from all, adding one adds it to all.
 */
export function TagsDialog({ servers, onOpenChange }: { servers: TaggedServer[]; onOpenChange: (open: boolean) => void }) {
  const [add, setAdd] = useState<string[]>([])
  const [remove, setRemove] = useState<string[]>([])
  const [typing, setTyping] = useState("")
  const input = useRef<HTMLInputElement>(null)
  const [error, setError] = useState<string>()
  const change = useChangeTags()
  const { data: all } = useQuery(allServersQuery)
  const count = (tag: string) => servers.filter((s) => s.tags.includes(tag)).length
  const shown = [...new Set([...servers.flatMap((s) => s.tags), ...add])].filter((tag) => !remove.includes(tag)).sort()
  const typed = typing.trim().toLowerCase()
  const suggestions = [...new Set(all?.flatMap((s) => s.tags))]
    .filter((tag) => !shown.includes(tag) && tag.startsWith(typed))
    .sort()
    .slice(0, 12)
  const tooMany = servers.some((s) => new Set([...s.tags.filter((tag) => !remove.includes(tag)), ...add]).size > maxTags)

  function addTag(tag: string) {
    if (!valid.test(tag)) {
      setError(t("Tags have up to 24 letters, digits, - and _."))
      return
    }
    setAdd((a) => [...new Set([...a, tag])])
    setRemove((r) => r.filter((x) => x !== tag))
    setTyping("")
    setError(undefined)
  }

  function removeTag(tag: string) {
    setAdd((a) => a.filter((x) => x !== tag))
    if (count(tag) > 0) setRemove((r) => [...r, tag])
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    if (typed) return addTag(typed)
    change.mutate(
      { servers, add, remove },
      {
        onSuccess: () => {
          toast.success(t("Saved the tags"))
          onOpenChange(false)
        },
        onError: (e) => setError(e.message),
      },
    )
  }

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent
        className="sm:max-w-lg"
        onOpenAutoFocus={(e) => {
          e.preventDefault()
          input.current?.focus()
        }}
      >
        <form onSubmit={submit} className="grid gap-5">
          <DialogHeader>
            <DialogTitle>
              {servers.length === 1
                ? t("Tags of {{name}}", { name: servers[0].name })
                : t("Tags of {{count}} servers", { count: servers.length })}
            </DialogTitle>
            <DialogDescription>{t("Tags such as lobby or bedwars help to find, filter and group servers.")}</DialogDescription>
          </DialogHeader>
          <div className="flex min-h-9 flex-wrap items-center gap-1.5 rounded-lg bg-muted/50 p-2">
            {shown.length === 0 && <span className="px-1 text-sm text-muted-foreground">{t("No tags")}</span>}
            {shown.map((tag) => {
              const n = add.includes(tag) ? servers.length : count(tag)
              return (
                <TagChip
                  key={tag}
                  tag={tag}
                  detail={servers.length > 1 && n < servers.length ? `${n}/${servers.length}` : undefined}
                  onRemove={() => removeTag(tag)}
                  className="h-6 bg-card text-xs ring-1 ring-foreground/8"
                />
              )
            })}
          </div>
          <div className="grid gap-2">
            <div className="flex gap-2">
              <Input
                ref={input}
                value={typing}
                maxLength={24}
                aria-label={t("New tag")}
                placeholder={t("Add a tag, e.g. lobby")}
                onChange={(e) => setTyping(e.target.value)}
              />
              <Button type="button" variant="outline" disabled={!typed} onClick={() => addTag(typed)}>
                <PlusIcon />
                {t("Add")}
              </Button>
            </div>
            {suggestions.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {suggestions.map((tag) => (
                  <button
                    key={tag}
                    type="button"
                    onClick={() => addTag(tag)}
                    className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground ring-1 ring-border outline-none hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <PlusIcon className="size-3" />
                    {tag}
                  </button>
                ))}
              </div>
            )}
            {(error || tooMany) && <FieldError>{error ?? t("A server can have up to {{count}} tags.", { count: maxTags })}</FieldError>}
          </div>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={change.isPending || tooMany || (add.length === 0 && remove.length === 0 && !typed)}>
              <TagIcon />
              {t("Save tags")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
