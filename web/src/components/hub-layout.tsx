import { Outlet } from "@tanstack/react-router"
import { t } from "i18next"
import type { ReactNode } from "react"
import type { Hub } from "@/components/navigation"
import { PageHeader } from "@/components/page-header"
import { TabLink } from "@/components/tab-link"
import { Tabs } from "@/components/tabs"
import { useAccess } from "@/features/access/use-access"
import type { settings } from "@/features/settings/tabs"

/** Pages under one header, e.g. those of the library, which switch with tabs; the tabs are child routes. */
export function HubLayout({ hub, children = <Outlet /> }: { hub: Hub | typeof settings; children?: ReactNode }) {
  const access = useAccess()
  const tabs = hub.tabs.filter((tab) => tab.visible(access))
  return (
    <>
      <PageHeader icon={hub.icon} tone={hub.tone} title={t(hub.label)} />
      {tabs.length > 0 && (
        <Tabs label={t(hub.label)}>
          {tabs.map((tab) => (
            <TabLink key={tab.to} to={tab.to} activeOptions={{ exact: "exact" in tab, includeSearch: false }}>
              <tab.icon className="size-4" weight="duotone" />
              {t(tab.label)}
            </TabLink>
          ))}
        </Tabs>
      )}
      {children}
    </>
  )
}

/** What a tab is about, and its actions, e.g. to create something; it opens the content of a hub's tab. */
export function TabIntro({ children, actions }: { children?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-6 flex flex-wrap items-center justify-between gap-x-8 gap-y-3">
      <p className="max-w-2xl text-sm text-pretty text-muted-foreground">{children}</p>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </div>
  )
}
