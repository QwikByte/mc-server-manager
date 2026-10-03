import { CrownIcon, PlusIcon, ShieldCheckIcon, TargetIcon, UsersIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { IconTile } from "@/components/icon-tile"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { describeScope, groupsQuery } from "./api"
import { useAccess } from "./use-access"

/** The Groups tab: what each group may do and where. */
export function GroupsPage() {
  const { can } = useAccess()
  const { data: groups, isPending, error } = useQuery(groupsQuery)

  if (isPending) return <Skeleton className="h-64 rounded-xl" />
  if (error) return <ErrorCallout error={error} />
  return (
    <>
      <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
        <p className="text-sm text-muted-foreground">{t("Groups give their members permissions, on all servers or only on some.")}</p>
        {can("groups.manage") && (
          <Button asChild>
            <Link to="/settings/groups/new">
              <PlusIcon />
              {t("New group")}
            </Link>
          </Button>
        )}
      </div>
      <ul className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {groups.map((group) => (
          <li key={group.id}>
            <Link
              to="/settings/groups/$groupId"
              params={{ groupId: group.id }}
              className="surface flex h-full flex-col gap-4 rounded-xl p-5 transition-all outline-none hover:-translate-y-0.5 hover:shadow-lg focus-visible:ring-2 focus-visible:ring-ring motion-reduce:hover:translate-y-0"
            >
              <div className="flex items-start gap-3">
                <IconTile icon={group.builtin ? CrownIcon : UsersIcon} tone={group.builtin ? "warning" : "violet"} />
                <div className="min-w-0 flex-1">
                  <p className="truncate font-semibold">{group.name}</p>
                  <p className="line-clamp-2 text-sm text-muted-foreground">{group.description || t("No description")}</p>
                </div>
              </div>
              <div className="mt-auto flex flex-wrap gap-2">
                <Chip icon={ShieldCheckIcon}>
                  {group.builtin
                    ? t("Every permission")
                    : t("{{count}} permissions", { count: group.permissions.length, defaultValue_one: "{{count}} permission" })}
                </Chip>
                <Chip icon={TargetIcon}>{describeScope(group)}</Chip>
                <Chip icon={UsersIcon}>
                  {t("{{count}} members", { count: group.members.length, defaultValue_one: "{{count}} member" })}
                </Chip>
              </div>
            </Link>
          </li>
        ))}
      </ul>
    </>
  )
}
