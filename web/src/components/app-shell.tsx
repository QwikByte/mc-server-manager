import { GraphIcon, HardDrivesIcon, SignOutIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link, Outlet, useNavigate } from "@tanstack/react-router"
import { Lamp } from "@/components/lamp"
import { Button } from "@/components/ui/button"
import { meQuery, useLogout } from "@/features/auth/api"

const navigation = [
  { to: "/nodes", label: "Nodes", icon: HardDrivesIcon },
  { to: "/networks", label: "Networks", icon: GraphIcon },
] as const

export function AppShell() {
  const { data: user } = useQuery(meQuery)
  const logout = useLogout()
  const navigate = useNavigate()

  return (
    <div className="flex min-h-svh flex-col md:flex-row">
      <aside className="flex shrink-0 items-center gap-6 border-b bg-rail px-4 py-3 md:sticky md:top-0 md:h-svh md:w-56 md:flex-col md:items-stretch md:border-r md:border-b-0 md:py-5">
        <Link to="/nodes" className="flex items-center gap-2.5">
          <Lamp state="on" />
          <span className="heading text-sm max-md:sr-only">MC Server Manager</span>
        </Link>
        <nav className="flex gap-1 md:flex-col" aria-label="Main">
          {navigation.map(({ to, label, icon: Icon }) => (
            <Link
              key={to}
              to={to}
              className="flex items-center gap-2.5 px-2 py-1.5 text-sm text-muted-foreground hover:bg-accent hover:text-foreground data-[status=active]:bg-accent data-[status=active]:font-medium data-[status=active]:text-foreground"
            >
              <Icon className="size-4" />
              {label}
            </Link>
          ))}
        </nav>
        <div className="ml-auto flex items-center gap-2 md:mt-auto md:ml-0 md:justify-between">
          <span className="truncate text-sm text-muted-foreground">{user?.username}</span>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Sign out"
            title="Sign out"
            onClick={() => logout.mutate(undefined, { onSettled: () => navigate({ to: "/login" }) })}
          >
            <SignOutIcon />
          </Button>
        </div>
      </aside>
      <main className="min-w-0 flex-1 px-4 py-6 md:px-10 md:py-10">
        <div className="mx-auto max-w-6xl">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
