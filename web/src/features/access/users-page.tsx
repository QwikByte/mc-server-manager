import { ShieldCheckIcon, UsersIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { ErrorCallout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { type Status, StatusBadge } from "@/components/status"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { meQuery } from "@/features/auth/api"
import { formatDate } from "@/lib/format"
import { groupsQuery, type User, usersQuery } from "./api"
import { InviteDialog } from "./invite-dialog"
import { useAccess } from "./use-access"
import { UserActions } from "./user-actions"

function statusOf(user: User): Status {
  if (user.disabled) return { tone: "neutral", label: "Disabled" }
  if (!user.passwordSet) return { tone: "warning", label: "Invited" }
  return { tone: "success", label: "Active" }
}

/** The Users tab: everyone who can sign in, with their groups. */
export function UsersPage() {
  const { can } = useAccess()
  const { data: me } = useQuery(meQuery)
  const { data: users, isPending, error } = useQuery(usersQuery)
  const { data: groups = [] } = useQuery(groupsQuery)
  const manage = can("users.manage")

  if (isPending) return <Skeleton className="h-64 rounded-xl" />
  if (error) return <ErrorCallout error={error} />
  return (
    <>
      <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
        <p className="text-sm text-muted-foreground">Users get their permissions from their groups.</p>
        {manage && <InviteDialog groups={groups} />}
      </div>
      <div className="surface overflow-hidden rounded-xl">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>User</TableHead>
              <TableHead>Groups</TableHead>
              <TableHead className="hidden md:table-cell">Added</TableHead>
              {manage && (
                <TableHead>
                  <span className="sr-only">Actions</span>
                </TableHead>
              )}
            </TableRow>
          </TableHeader>
          <TableBody>
            {users.map((user) => (
              <TableRow key={user.id} className={user.disabled ? "text-muted-foreground" : undefined}>
                <TableCell>
                  <span className="flex items-center gap-3">
                    <span
                      aria-hidden
                      className="grid size-8 shrink-0 place-items-center rounded-full bg-linear-to-br from-violet-500 to-sky-500 text-xs font-bold text-white uppercase"
                    >
                      {user.username.charAt(0)}
                    </span>
                    <span className="font-medium">{user.username}</span>
                    {user.id === me?.id && <Chip>You</Chip>}
                    <StatusBadge status={statusOf(user)} />
                    {user.mfa && (
                      <Chip icon={ShieldCheckIcon} className="max-sm:hidden">
                        2FA
                      </Chip>
                    )}
                  </span>
                </TableCell>
                <TableCell>
                  <span className="flex flex-wrap gap-1.5">
                    {user.groups.length === 0 ? (
                      <span className="text-xs text-muted-foreground">No groups, no permissions</span>
                    ) : (
                      user.groups.map((id) => (
                        <Chip key={id} icon={UsersIcon}>
                          {groups.find((g) => g.id === id)?.name ?? "…"}
                        </Chip>
                      ))
                    )}
                  </span>
                </TableCell>
                <TableCell className="hidden md:table-cell">{formatDate(user.createdAt)}</TableCell>
                {manage && (
                  <TableCell>
                    <UserActions user={user} groups={groups} self={user.id === me?.id} />
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </>
  )
}
