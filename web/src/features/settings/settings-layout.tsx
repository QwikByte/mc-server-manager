import { GearSixIcon } from "@phosphor-icons/react"
import { Navigate, Outlet, useLocation } from "@tanstack/react-router"
import { t } from "i18next"
import { EmptyState } from "@/components/empty-state"
import { HubLayout } from "@/components/hub-layout"
import { useAccess } from "@/features/access/use-access"
import { settings } from "./tabs"

/** Header and tabs of the settings; the tabs are child routes. */
export function SettingsLayout() {
  const access = useAccess()
  const { pathname } = useLocation()
  const visible = settings.tabs.filter((tab) => tab.visible(access))
  // Without the General tab, the settings open with the first tab the user may see.
  if (pathname.replace(/\/$/, "") === "/settings" && !access.can("settings.view") && visible.length > 0) {
    return <Navigate to={visible[0].to} replace />
  }
  return (
    <HubLayout hub={settings}>
      {visible.length > 0 ? (
        <Outlet />
      ) : (
        <EmptyState
          icon={GearSixIcon}
          tone="neutral"
          title={t("Nothing to see here")}
          description={t("Your groups don't include permissions for the settings.")}
        />
      )}
    </HubLayout>
  )
}
