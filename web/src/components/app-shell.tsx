import { ListIcon, SidebarSimpleIcon, SignOutIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link, Outlet, useMatches, useMatchRoute, useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { LayoutGroup, motion } from "motion/react"
import { Fragment, type ReactElement, useEffect, useId, useState } from "react"
import { LanguageMenu } from "@/components/language-menu"
import { Logo } from "@/components/logo"
import { navigation } from "@/components/navigation"
import { StatusDot } from "@/components/status"
import { ThemeToggle } from "@/components/theme-toggle"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetTitle, SheetTrigger } from "@/components/ui/sheet"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { useAccess } from "@/features/access/use-access"
import { meQuery, useLogout, useSetLanguage } from "@/features/auth/api"
import { LogAlerts } from "@/features/logs/log-alerts"
import { Activity } from "@/features/operations/activity"
import { PaletteButton } from "@/features/palette/command-palette"
import { usePinned } from "@/features/preferences/api"
import { allServersQuery } from "@/features/servers/api"
import { serverLook, statusOf } from "@/features/servers/server-types"
import { UpdateBanner } from "@/features/updates/update-banner"
import { chooseLanguage, chosenLanguage } from "@/lib/i18n"
import { cn } from "@/lib/utils"

// Whether the sidebar shows only its icons; each browser keeps it, as it depends on the screen.
function readCollapsed() {
  try {
    return localStorage.getItem("sidebar") === "collapsed"
  } catch {
    return false
  }
}

function storeCollapsed(collapsed: boolean) {
  try {
    if (collapsed) localStorage.setItem("sidebar", "collapsed")
    else localStorage.removeItem("sidebar")
  } catch {
    // The choice then only lasts until the page is reloaded.
  }
}

/**
 * A sidebar on large screens, which folds to its icons; on small ones, a bar at the top whose
 * menu holds the navigation.
 */
export function AppShell() {
  const { data: user } = useQuery(meQuery)
  const access = useAccess()
  const logout = useLogout()
  const navigate = useNavigate()
  const [menu, setMenu] = useState(false)
  const [collapsed, setCollapsed] = useState(readCollapsed)
  const setLanguage = useSetLanguage()
  // The language the user chose applies in every browser, once signed in.
  useEffect(() => {
    if (user?.language) chooseLanguage(user.language)
  }, [user?.language])

  const folded = collapsed && "md:sr-only"
  const preferences = (
    <>
      <LanguageMenu value={user?.language ?? chosenLanguage()} onChoose={(language) => setLanguage.mutate(language)} />
      <ThemeToggle className={cn("flex-1", collapsed && "md:flex-col md:rounded-2xl")} />
    </>
  )
  const toggleLabel = collapsed ? t("Expand the sidebar") : t("Collapse the sidebar")

  return (
    <div className="flex min-h-svh flex-col md:flex-row">
      <aside
        className={cn(
          "sticky top-0 z-30 flex shrink-0 items-center gap-1.5 border-b bg-sidebar/80 px-3 py-2.5 backdrop-blur-xl md:h-svh md:flex-col md:items-stretch md:gap-6 md:overflow-x-hidden md:overflow-y-auto md:border-r md:border-b-0 md:py-6 md:transition-[width] md:duration-300 md:ease-out",
          collapsed ? "md:w-18 md:px-3" : "md:w-64 md:px-4",
        )}
      >
        <Sheet open={menu} onOpenChange={setMenu}>
          <SheetTrigger asChild>
            <Button variant="ghost" size="icon" aria-label={t("Menu")} className="md:hidden">
              <ListIcon />
            </Button>
          </SheetTrigger>
          <SheetContent side="left" aria-describedby={undefined} className="w-72 gap-8 overflow-y-auto bg-sidebar px-4 py-6">
            <SheetTitle className="flex items-center gap-3 px-2 text-sm font-bold tracking-tight">
              <Logo />
              {t("Noryx")}
            </SheetTitle>
            <MainNav onNavigate={() => setMenu(false)} />
            {/* On small screens, the bar at the top has no room for them. */}
            <div className="mt-auto flex items-center gap-2">{preferences}</div>
          </SheetContent>
        </Sheet>
        <div className={cn("flex items-center gap-2", collapsed && "md:flex-col")}>
          <Link to="/" className="flex min-w-0 flex-1 items-center gap-3 rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring md:px-2">
            <Logo />
            <span className={cn("leading-tight whitespace-nowrap max-md:sr-only", folded)}>
              <span className="block text-sm font-bold tracking-tight">{t("Noryx")}</span>
              <span className="block text-xs text-muted-foreground">{t("Admin panel")}</span>
            </span>
          </Link>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={toggleLabel}
            title={toggleLabel}
            aria-expanded={!collapsed}
            onClick={() => {
              setCollapsed(!collapsed)
              storeCollapsed(!collapsed)
            }}
            className="text-muted-foreground max-md:hidden"
          >
            <SidebarSimpleIcon />
          </Button>
        </div>
        <PaletteButton
          folded={collapsed}
          className={cn("max-md:size-9 max-md:justify-center max-md:px-0 md:-mb-3", collapsed && "md:size-9 md:self-center md:px-0")}
        />
        <MainNav className="max-md:hidden" collapsed={collapsed} />
        <div className={cn("ml-auto flex items-center gap-1.5 md:mt-auto md:ml-0 md:flex-col md:items-stretch md:gap-3", collapsed && "md:items-center")}>
          <div className={cn("flex items-center gap-1.5 md:gap-2", collapsed && "md:flex-col")}>
            <Activity />
            {access.canSomewhere("logs.view") && <LogAlerts />}
            <div className="contents max-md:hidden">{preferences}</div>
          </div>
          <div className={cn("flex items-center gap-1.5 md:gap-3 md:rounded-xl md:bg-muted/60 md:p-2", collapsed && "md:flex-col md:gap-2")}>
            <Hint label={collapsed ? user?.username : undefined}>
              <Link
                to="/account"
                className="group flex min-w-0 flex-1 items-center gap-3 rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <span
                  aria-hidden
                  className="grid size-8 shrink-0 place-items-center rounded-full bg-linear-to-br from-violet-500 to-sky-500 text-xs font-bold text-white uppercase ring-primary/40 ring-offset-2 ring-offset-sidebar transition-shadow group-hover:ring-2"
                >
                  {user?.username.charAt(0)}
                </span>
                <span className={cn("min-w-0 flex-1 leading-tight max-md:sr-only", folded)}>
                  <span className="block truncate text-sm font-semibold group-hover:text-primary group-data-[status=active]:text-primary">
                    {user?.username}
                  </span>
                  <span className="block text-xs text-muted-foreground">{t("Your account")}</span>
                </span>
              </Link>
            </Hint>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={t("Sign out")}
              title={t("Sign out")}
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
          <Page />
        </div>
      </main>
    </div>
  )
}

/** The page fades in when another one opens, but not when only its tab or search changes. */
function Page() {
  const page = useMatches({ select: (matches) => matches[2]?.pathname })
  return (
    <motion.div key={page} initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.25, ease: "easeOut" }}>
      <Outlet />
    </motion.div>
  )
}

/** Names what a folded sidebar only shows as an icon. */
function Hint({ label, children }: { label?: string; children: ReactElement }) {
  if (!label) return children
  return (
    <Tooltip>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      <TooltipContent side="right">{label}</TooltipContent>
    </Tooltip>
  )
}

const linkClass =
  "relative isolate flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors outline-none hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring data-[status=active]:text-primary"

/** The highlight of the active link, which glides over to the next one. */
const ActiveMark = () => (
  <motion.span
    layoutId="active"
    aria-hidden
    className="absolute inset-0 -z-10 rounded-lg bg-primary/10"
    transition={{ type: "spring", bounce: 0.15, duration: 0.45 }}
  />
)

/** The sections of the panel the user may see, and the servers they pinned. */
function MainNav({ className, collapsed, onNavigate }: { className?: string; collapsed?: boolean; onNavigate?: () => void }) {
  const access = useAccess()
  // Servers open under their node, but belong to the servers section.
  const onServer = !!useMatchRoute()({ to: "/nodes/$nodeId/servers/$serverId", fuzzy: true })
  const heading = cn(
    "px-3 pb-2 text-[0.6875rem] font-semibold tracking-wider whitespace-nowrap text-muted-foreground uppercase not-first:pt-5",
    collapsed && "md:sr-only",
  )
  return (
    // Each navigation, in the sidebar and in the menu, glides its own highlight.
    <LayoutGroup id={useId()}>
      <nav aria-label={t("Main")} className={cn("flex flex-col gap-1", collapsed && "md:items-center", className)}>
        {navigation
          .map(({ title, links }) => ({ title, links: links.filter((l) => l.visible(access)) }))
          .filter(({ links }) => links.length > 0)
          .map(({ title, links }) => (
            <Fragment key={title}>
              <p className={heading}>{t(title)}</p>
              {links.map(({ to, label, icon: Icon }) => (
                <Hint key={to} label={collapsed ? t(label) : undefined}>
                  <Link
                    to={to}
                    activeOptions={{ exact: to === "/" || (to === "/nodes" && onServer) }}
                    {...(to === "/servers" && onServer && { "data-status": "active", "aria-current": "page" as const })}
                    onClick={onNavigate}
                    className={linkClass}
                  >
                    {({ isActive }) => (
                      <>
                        {(isActive || (to === "/servers" && onServer)) && <ActiveMark />}
                        <Icon className="size-5 shrink-0" weight="duotone" />
                        <span className={cn("whitespace-nowrap", collapsed && "md:sr-only")}>{t(label)}</span>
                      </>
                    )}
                  </Link>
                </Hint>
              ))}
            </Fragment>
          ))}
        <PinnedServers heading={heading} collapsed={collapsed} onNavigate={onNavigate} />
      </nav>
    </LayoutGroup>
  )
}

/** The servers the user pinned, with their state; the list of servers is only loaded once there are some. */
function PinnedServers({ heading, collapsed, onNavigate }: { heading: string; collapsed?: boolean; onNavigate?: () => void }) {
  const { pinned } = usePinned()
  const { data: all = [] } = useQuery({ ...allServersQuery, enabled: pinned.length > 0 })
  const servers = pinned.flatMap((p) => all.find((s) => s.id === p.serverId) ?? [])
  if (servers.length === 0) return null
  return (
    <>
      <p className={heading}>{t("Pinned")}</p>
      {servers.map((s) => {
        const status = statusOf(s)
        const { icon: Icon } = serverLook(s.type)
        return (
          <Hint key={s.id} label={collapsed ? `${s.name} · ${t(status.label)}` : undefined}>
            <Link
              to="/nodes/$nodeId/servers/$serverId"
              params={{ nodeId: s.nodeId, serverId: s.id }}
              activeOptions={{ includeSearch: false }}
              onClick={onNavigate}
              className={linkClass}
            >
              <span className="relative grid size-5 shrink-0 place-items-center">
                <Icon className="size-5" weight="duotone" />
                <StatusDot status={status} className="absolute -right-0.5 -bottom-0.5 rounded-full ring-2 ring-sidebar" />
              </span>
              <span className={cn("truncate", collapsed && "md:sr-only")}>{s.name}</span>
              <span className="sr-only">{t(status.label)}</span>
            </Link>
          </Hint>
        )
      })}
    </>
  )
}
