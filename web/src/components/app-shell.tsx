import { ListIcon, SignOutIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link, Outlet, useNavigate } from "@tanstack/react-router"
import { Fragment, useState } from "react"
import { Logo } from "@/components/logo"
import { navigation } from "@/components/navigation"
import { ThemeToggle } from "@/components/theme-toggle"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetTitle, SheetTrigger } from "@/components/ui/sheet"
import { useAccess } from "@/features/access/use-access"
import { meQuery, useLogout } from "@/features/auth/api"
import { LogAlerts } from "@/features/logs/log-alerts"
import { UpdateBanner } from "@/features/updates/update-banner"
import { cn } from "@/lib/utils"

/** A sidebar on large screens; on small ones, a bar at the top whose menu holds the navigation. */
export function AppShell() {
  const { data: user } = useQuery(meQuery)
  const access = useAccess()
  const logout = useLogout()
  const navigate = useNavigate()
  const [menu, setMenu] = useState(false)

  return (
    <div className="flex min-h-svh flex-col md:flex-row">
      <aside className="sticky top-0 z-30 flex shrink-0 items-center gap-2 border-b bg-sidebar/80 px-3 py-2.5 backdrop-blur-xl md:h-svh md:w-64 md:flex-col md:items-stretch md:gap-8 md:border-r md:border-b-0 md:px-4 md:py-6">
        <Sheet open={menu} onOpenChange={setMenu}>
          <SheetTrigger asChild>
            <Button variant="ghost" size="icon" aria-label="Menu" className="md:hidden">
              <ListIcon />
            </Button>
          </SheetTrigger>
          <SheetContent side="left" aria-describedby={undefined} className="w-72 gap-8 overflow-y-auto bg-sidebar px-4 py-6">
            <SheetTitle className="flex items-center gap-3 px-2 text-sm font-bold tracking-tight">
              <Logo />
              MC Server Manager
            </SheetTitle>
            <MainNav onNavigate={() => setMenu(false)} />
          </SheetContent>
        </Sheet>
        <Link to="/" className="flex items-center gap-3 rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring md:px-2">
          <Logo />
          <span className="leading-tight max-md:sr-only">
            <span className="block text-sm font-bold tracking-tight">MC Server Manager</span>
            <span className="block text-xs text-muted-foreground">Admin panel</span>
          </span>
        </Link>
        <MainNav className="max-md:hidden" />
        <div className="ml-auto flex items-center gap-2 md:mt-auto md:ml-0 md:flex-col md:items-stretch md:gap-3">
          <div className="flex items-center gap-2">
            {access.canSomewhere("logs.view") && <LogAlerts />}
            <ThemeToggle className="md:flex-1" />
          </div>
          <div className="flex items-center gap-2 md:gap-3 md:rounded-xl md:bg-muted/60 md:p-2">
            <Link to="/account" className="group flex min-w-0 flex-1 items-center gap-3 rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-ring">
              <span
                aria-hidden
                className="grid size-8 shrink-0 place-items-center rounded-full bg-linear-to-br from-violet-500 to-sky-500 text-xs font-bold text-white uppercase ring-primary/40 ring-offset-2 ring-offset-sidebar transition-shadow group-hover:ring-2"
              >
                {user?.username.charAt(0)}
              </span>
              <span className="min-w-0 flex-1 leading-tight max-md:sr-only">
                <span className="block truncate text-sm font-semibold group-hover:text-primary group-data-[status=active]:text-primary">
                  {user?.username}
                </span>
                <span className="block text-xs text-muted-foreground">Your account</span>
              </span>
            </Link>
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
          {access.admin && <UpdateBanner />}
          <Outlet />
        </div>
      </main>
    </div>
  )
}

/** The sections of the panel the user may see. */
function MainNav({ className, onNavigate }: { className?: string; onNavigate?: () => void }) {
  const access = useAccess()
  return (
    <nav aria-label="Main" className={cn("flex flex-col gap-1", className)}>
      {navigation
        .map(({ title, links }) => ({ title, links: links.filter((l) => l.visible(access)) }))
        .filter(({ links }) => links.length > 0)
        .map(({ title, links }) => (
          <Fragment key={title}>
            <p className="px-3 pb-2 text-[0.6875rem] font-semibold tracking-wider text-muted-foreground uppercase not-first:pt-5">{title}</p>
            {links.map(({ to, label, icon: Icon }) => (
              <Link
                key={to}
                to={to}
                onClick={onNavigate}
                className="group flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground data-[status=active]:bg-primary/10 data-[status=active]:text-primary"
              >
                <Icon className="size-5" weight="duotone" />
                {label}
              </Link>
            ))}
          </Fragment>
        ))}
    </nav>
  )
}
