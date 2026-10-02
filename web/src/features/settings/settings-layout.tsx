import { GearSixIcon, HardDrivesIcon, SlidersHorizontalIcon, TerminalWindowIcon, UsersIcon } from "@phosphor-icons/react"
import { Outlet } from "@tanstack/react-router"
import { PageHeader } from "@/components/page-header"
import { TabLink } from "@/components/tab-link"
import { Tabs, UpcomingTab } from "@/components/tabs"

const tabs = [
  { to: "/settings", label: "General", icon: SlidersHorizontalIcon, exact: true },
  { to: "/settings/agents", label: "Agents", icon: HardDrivesIcon, exact: false },
  { to: "/settings/terminal", label: "Terminal", icon: TerminalWindowIcon, exact: false },
] as const

/** Header and tabs of the settings; the tabs are child routes. */
export function SettingsLayout() {
  return (
    <>
      <PageHeader
        icon={GearSixIcon}
        tone="neutral"
        title="Settings"
        description="Configure the master and its agents, and run their commands."
      />
      <Tabs label="Settings">
        {tabs.map(({ to, label, icon: Icon, exact }) => (
          <TabLink key={to} to={to} activeOptions={{ exact, includeSearch: false }}>
            <Icon className="size-4" weight="duotone" />
            {label}
          </TabLink>
        ))}
        <UpcomingTab>
          <UsersIcon className="size-4" weight="duotone" />
          Users
        </UpcomingTab>
      </Tabs>
      <Outlet />
    </>
  )
}
