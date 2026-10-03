import { GearSixIcon } from "@phosphor-icons/react"
import { Navigate, Outlet, useLocation } from "@tanstack/react-router"
import { t } from "i18next"
import { EmptyState } from "@/components/empty-state"
import { PageHeader } from "@/components/page-header"
import { TabLink } from "@/components/tab-link"
import { Tabs } from "@/components/tabs"
import { useAccess } from "@/features/access/use-access"
import { settingsTabs as tabs } from "./tabs"

/** Header and tabs of the settings; the tabs are child routes. */
export function SettingsLayout() {
  const access = useAccess()
  const { pathname } = useLocation()
  const visible = tabs.filter((tab) => tab.visible(access))
  // Without the General tab, the settings open with the first tab the user may see.
  if (pathname.replace(/\/$/, "") === "/settings" && !access.can("settings.view") && visible.length > 0) {
    return <Navigate to={visible[0].to} replace />
  }

  return (
    <>
      <PageHeader
        icon={GearSixIcon}
        tone="neutral"
        title={t("Settings")}
        description={t("Configure the master and its agents, run their commands and manage who may do what.")}
      />
      {visible.length === 0 ? (
        <EmptyState
          icon={GearSixIcon}
          tone="neutral"
          title={t("Nothing to see here")}
          description={t("Your groups don't include permissions for the settings.")}
        />
      ) : (
        <>
          <Tabs label={t("Settings")}>
            {visible.map(({ to, label, icon: Icon, exact }) => (
              <TabLink key={to} to={to} activeOptions={{ exact, includeSearch: false }}>
                <Icon className="size-4" weight="duotone" />
                {t(label)}
              </TabLink>
            ))}
          </Tabs>
          <Outlet />
        </>
      )}
    </>
  )
}
