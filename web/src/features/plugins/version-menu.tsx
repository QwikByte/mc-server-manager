import { CheckIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { type ReactNode, useState } from "react"
import { Pill } from "@/components/status"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { formatDate } from "@/lib/format"
import { type ProjectVersion, versionsQuery } from "./api"

/**
 * A menu of the versions of a project that run on a server type and Minecraft version, the newest first, betas and
 * alphas included. With onNewest, it offers the newest suitable release first.
 */
export function VersionMenu({
  project,
  type,
  version,
  current,
  onNewest,
  onPick,
  children,
}: {
  project: string
  type: string
  version: string
  /** The ID of the installed or chosen version. */
  current?: string
  onNewest?: () => void
  onPick: (version: ProjectVersion) => void
  children: ReactNode
}) {
  const [open, setOpen] = useState(false)
  const { data, error, isPending } = useQuery({ ...versionsQuery(project, type, version), enabled: open })
  const mark = (on: boolean) => <CheckIcon className={on ? undefined : "invisible"} />

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>{children}</DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="max-h-80 w-72">
        {onNewest && (
          <>
            <DropdownMenuItem onSelect={onNewest}>
              <span className="flex-1">{t("Newest suitable release")}</span>
              {mark(!current)}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
          </>
        )}
        <DropdownMenuLabel>{t("Suitable versions")}</DropdownMenuLabel>
        {isPending ? (
          [0, 1, 2].map((i) => <Skeleton key={i} className="m-1 h-6" />)
        ) : error ? (
          <p className="px-1.5 py-1 text-xs text-destructive">{error.message}</p>
        ) : data.length === 0 ? (
          <p className="px-1.5 py-1 text-xs text-muted-foreground">{t("No suitable version.")}</p>
        ) : (
          data.map((v) => (
            <DropdownMenuItem key={v.id} onSelect={() => onPick(v)}>
              <span className="truncate font-mono text-xs">{v.number}</span>
              {v.channel !== "release" && <Pill tone={v.channel === "beta" ? "warning" : "destructive"}>{v.channel === "beta" ? t("Beta") : t("Alpha")}</Pill>}
              <span className="ml-auto shrink-0 text-xs text-muted-foreground">{formatDate(v.published)}</span>
              {mark(v.id === current)}
            </DropdownMenuItem>
          ))
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
