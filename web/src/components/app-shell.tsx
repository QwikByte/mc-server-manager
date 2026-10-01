import { GraphIcon, HardDrivesIcon, SignOutIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link, Outlet, useNavigate } from "@tanstack/react-router"
import { Logo } from "@/components/logo"
import { ThemeToggle } from "@/components/theme-toggle"
import { Button } from "@/components/ui/button"
import { meQuery, useLogout } from "@/features/auth/api"

const navigation = [
  { to: "/nodes", label: "Nodes", icon: HardDrivesIcon },
  { to: "/networks", label: "Networks", icon: GraphIcon },
] as const

/** A sidebar on large screens, a bar at the top on small ones. */
export function AppShell() {
  const { data: user } = useQuery(meQuery)
  const logout = useLogout()
  const navigate = useNavigate()

  return (
    <div className="flex min-h-svh flex-col md:flex-row">
      <aside className="sticky top-0 z-30 flex shrink-0 items-center gap-2 border-b bg-sidebar/80 px-3 py-2.5 backdrop-blur-xl md:h-svh md:w-64 md:flex-col md:items-stretch md:gap-8 md:border-r md:border-b-0 md:px-4 md:py-6">
        <Link to="/nodes" className="flex items-center gap-3 rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring md:px-2">
          <Logo />
          <span className="leading-tight max-md:sr-only">
            <span className="block text-sm font-bold tracking-tight">MC Server Manager</span>
            <span className="block text-xs text-muted-foreground">Admin panel</span>
          </span>
        </Link>
        <nav aria-label="Main" className="flex gap-1 md:flex-col">
          <p className="px-3 pb-2 text-[0.6875rem] font-semibold tracking-wider text-muted-foreground uppercase max-md:hidden">Manage</p>
          {navigation.map(({ to, label, icon: Icon }) => (
            <Link
              key={to}
              to={to}
              className="group flex items-center gap-3 rounded-lg px-2.5 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground data-[status=active]:bg-primary/10 data-[status=active]:text-primary md:px-3"
            >
              <Icon className="size-5" weight="duotone" />
              <span className="max-sm:sr-only">{label}</span>
            </Link>
          ))}
        </nav>
        <div className="ml-auto flex items-center gap-2 md:mt-auto md:ml-0 md:flex-col md:items-stretch md:gap-3">
          <ThemeToggle className="md:w-full" />
          <div className="flex items-center gap-3 md:rounded-xl md:bg-muted/60 md:p-2">
            <span
              aria-hidden
              className="grid size-8 shrink-0 place-items-center rounded-full bg-linear-to-br from-violet-500 to-sky-500 text-xs font-bold text-white uppercase max-md:hidden"
            >
              {user?.username.charAt(0)}
            </span>
            <span className="min-w-0 flex-1 leading-tight max-md:hidden">
              <span className="block truncate text-sm font-semibold">{user?.username}</span>
              <span className="block text-xs text-muted-foreground">Signed in</span>
            </span>
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
        </div>
      </aside>
      <main className="min-w-0 flex-1 px-4 py-6 sm:px-6 md:px-10 md:py-10">
        <div className="mx-auto max-w-6xl">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
