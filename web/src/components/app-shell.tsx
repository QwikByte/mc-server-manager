import { ListIcon, SidebarSimpleIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link, Outlet, useLocation, useMatches } from "@tanstack/react-router"
import { t } from "i18next"
import { LayoutGroup, motion } from "motion/react"
import { type ReactElement, useEffect, useId, useState } from "react"
import { ConnectionBanner } from "@/components/connection-banner"
import { Logo } from "@/components/logo"
import { navigation } from "@/components/navigation"
import { StatusDot } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetTitle, SheetTrigger } from "@/components/ui/sheet"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { useAccess } from "@/features/access/use-access"
import { AccountMenu } from "@/features/auth/account-menu"
import { meQuery } from "@/features/auth/api"
import { LogAlerts } from "@/features/logs/log-alerts"
import { Activity } from "@/features/operations/activity"
import { PaletteButton } from "@/features/palette/command-palette"
import { useApplySettings, usePinned } from "@/features/preferences/api"
import { allServersQuery } from "@/features/servers/api"
import { serverLook, statusOf } from "@/features/servers/server-types"
import { UpdateBanner } from "@/features/updates/update-banner"
import { chooseLanguage } from "@/lib/i18n"
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
  const [menu, setMenu] = useState(false)
  const [collapsed, setCollapsed] = useState(readCollapsed)
  // The language and settings the user chose apply in every browser, once signed in.
  useEffect(() => {
    if (user?.language) chooseLanguage(user.language)
  }, [user?.language])
  useApplySettings()

  const toggleLabel = collapsed ? t("Expand the sidebar") : t("Collapse the sidebar")

  return (
    <div className="flex min-h-svh flex-col md:flex-row">
      <aside
        className={cn(
          "sticky top-0 z-30 flex shrink-0 items-center gap-1.5 border-b bg-sidebar/80 px-3 py-2.5 backdrop-blur-xl md:h-svh md:flex-col md:items-stretch md:gap-4 md:overflow-x-hidden md:overflow-y-auto md:border-r md:border-b-0 md:py-4 md:transition-[width] md:duration-300 md:ease-out",
          collapsed ? "md:w-17" : "md:w-60",
        )}
      >
        <Sheet open={menu} onOpenChange={setMenu}>
          <SheetTrigger asChild>
            <Button variant="ghost" size="icon" aria-label={t("Menu")} className="md:hidden">
              <ListIcon />
            </Button>
          </SheetTrigger>
          <SheetContent side="left" aria-describedby={undefined} className="w-72 gap-6 overflow-y-auto bg-sidebar px-3 py-5">
            <SheetTitle className="flex items-center gap-2.5 px-2 text-base font-bold tracking-tight">
              <Logo className="size-8" />
              {t("Noryx")}
            </SheetTitle>
            <MainNav onNavigate={() => setMenu(false)} />
          </SheetContent>
        </Sheet>

        <div className={cn("flex items-center gap-2 md:px-1", collapsed && "md:flex-col md:px-0")}>
          <Link to="/" className="flex min-w-0 flex-1 items-center gap-2.5 rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <Logo className="size-8" />
            <span className={cn("text-base font-bold tracking-tight whitespace-nowrap max-md:sr-only", collapsed && "md:sr-only")}>
              {t("Noryx")}
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
          className={cn("max-md:ml-auto max-md:size-9 max-md:justify-center max-md:px-0", collapsed && "md:size-9 md:self-center md:px-0")}
        />
        <MainNav className="max-md:hidden md:flex-1" collapsed={collapsed} />

        {/* Operations, warnings and the account: at the foot of the sidebar, at the end of the bar on small screens. */}
        <div className={cn("flex items-center gap-1 md:border-t md:pt-3", collapsed && "md:flex-col md:gap-2")}>
          <AccountMenu folded={collapsed} className={cn("max-md:order-last max-md:p-0.5", !collapsed && "md:min-w-0 md:flex-1")} />
          <div className={cn("flex items-center gap-1 max-md:contents", collapsed && "md:order-first md:flex-col")}>
            <Activity />
            {access.canSomewhere("logs.view") && <LogAlerts />}
          </div>
        </div>
      </aside>
      <main className="min-w-0 flex-1 px-4 py-6 sm:px-6 md:px-10 md:py-9">
        <div className="mx-auto max-w-6xl">
          <ConnectionBanner />
          {access.admin && <UpdateBanner />}
          <Page />
        </div>
      </main>
    </div>
  )
}

/**
 * The page fades in when another one opens, but not when only its tab or search changes; the
 * tabs of a hub, e.g. of the settings, are one page.
 */
function Page() {
  const page = useMatches({ select: (matches) => matches[2] && `${matches[2].routeId}${matches[2].pathname}` })
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
  "relative isolate flex h-9 shrink-0 items-center gap-3 rounded-lg px-2.5 text-sm font-medium text-muted-foreground transition-colors outline-none hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring data-[status=active]:text-primary"

/** The highlight of the active link, which glides over to the next one. */
const ActiveMark = () => (
  <motion.span
    layoutId="active"
    aria-hidden
    className="absolute inset-0 -z-10 rounded-lg bg-primary/10"
    transition={{ type: "spring", bounce: 0.15, duration: 0.45 }}
  />
)

const heading = "px-2.5 pt-4 pb-1.5 text-[0.6875rem] font-semibold tracking-wider whitespace-nowrap text-muted-foreground/80 uppercase"

type Entry = (typeof navigation.main)[number] | (typeof navigation.system)[number]

/**
 * The sections of the panel the user may see, the servers they pinned, and those of the system
 * at the foot. A hub links its first tab the user may see and stays active on all of them.
 */
function MainNav({ className, collapsed, onNavigate }: { className?: string; collapsed?: boolean; onNavigate?: () => void }) {
  const access = useAccess()
  const pathname = useLocation({ select: (l) => l.pathname })
  const under = (path: string) => pathname === path || pathname.startsWith(`${path}/`)
  // Servers open under their node, but belong to the servers section.
  const onServer = /^\/nodes\/[^/]+\/servers\//.test(pathname)
  const active = (entry: Entry) => {
    if ("tabs" in entry) return entry.tabs.some((tab) => under(tab.to))
    if (entry.to === "/") return pathname === "/"
    if (entry.to === "/servers") return under("/servers") || onServer
    if (entry.to === "/nodes") return under("/nodes") && !onServer
    return under(entry.to)
  }

  const links = (entries: readonly Entry[]) =>
    entries.map((entry) => {
      const to = "tabs" in entry ? entry.tabs.find((tab) => tab.visible(access))?.to : entry.visible(access) && entry.to
      if (!to) return null
      const isActive = active(entry)
      return (
        <Hint key={entry.label} label={collapsed ? t(entry.label) : undefined}>
          <Link
            to={to}
            activeOptions={{ exact: true, includeSearch: false }}
            {...(isActive && { "data-status": "active", "aria-current": "page" as const })}
            onClick={onNavigate}
            className={linkClass}
          >
            {isActive && <ActiveMark />}
            <entry.icon className="size-5 shrink-0" weight={isActive ? "fill" : "duotone"} />
            <span className={cn("truncate", collapsed && "md:sr-only")}>{t(entry.label)}</span>
          </Link>
        </Hint>
      )
    })

  return (
    // Each navigation, in the sidebar and in the menu, glides its own highlight.
    <LayoutGroup id={useId()}>
      <nav aria-label={t("Main")} className={cn("flex flex-col gap-0.5", collapsed && "md:items-center", className)}>
        {links(navigation.main)}
        <PinnedServers collapsed={collapsed} onNavigate={onNavigate} />
        <div aria-hidden className="min-h-4 flex-1" />
        {links(navigation.system)}
      </nav>
    </LayoutGroup>
  )
}

/** The servers the user pinned, with their state; the list of servers is only loaded once there are some. */
function PinnedServers({ collapsed, onNavigate }: { collapsed?: boolean; onNavigate?: () => void }) {
  const { pinned } = usePinned()
  const { data: all = [] } = useQuery({ ...allServersQuery, enabled: pinned.length > 0 })
  const servers = pinned.flatMap((p) => all.find((s) => s.id === p.serverId) ?? [])
  if (servers.length === 0) return null
  return (
    <>
      <p className={cn(heading, collapsed && "md:sr-only")}>{t("Pinned")}</p>
      {collapsed && <hr aria-hidden className="my-2 w-6 max-md:hidden" />}
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
              {/* The servers section keeps the highlight; a pinned server only takes its colour. */}
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
