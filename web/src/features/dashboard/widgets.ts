import {
  CalendarCheckIcon,
  ChartBarIcon,
  ChartLineIcon,
  GraphIcon,
  HardDrivesIcon,
  type Icon,
  LightningIcon,
  PushPinIcon,
  ScrollIcon,
  UsersThreeIcon,
  WarningCircleIcon,
} from "@phosphor-icons/react"
import type { ReactNode } from "react"
import type { Access } from "@/features/access/use-access"
import type { Widget } from "@/features/preferences/api"
import { msg } from "@/lib/i18n"
import { RecentActivity } from "./activity"
import { Figures } from "./figures"
import { Attention, Networks, Nodes, Pinned, TopServers } from "./lists"
import { QuickActions } from "./quick-actions"
import { Resources } from "./resources"
import { Schedules } from "./schedules"

export interface WidgetDef {
  id: string
  /** In English, marked with msg; translated where it shows. */
  title: string
  icon: Icon
  /** How many of the three columns it spans unless the user chose otherwise. */
  columns: Widget["columns"]
  visible: (a: Access) => boolean
  Component: (props: { title: string }) => ReactNode
}

const seesServers = (a: Access) => a.canSomewhere("servers.view")

/** The widgets of the overview, in their default order. */
export const widgets: WidgetDef[] = [
  { id: "figures", title: msg("Key figures"), icon: ChartBarIcon, columns: 3, visible: () => true, Component: Figures },
  { id: "attention", title: msg("Needs attention"), icon: WarningCircleIcon, columns: 2, visible: () => true, Component: Attention },
  {
    id: "nodes",
    title: msg("Nodes"),
    icon: HardDrivesIcon,
    columns: 1,
    visible: (a) => a.canSomewhere("nodes.view") || seesServers(a),
    Component: Nodes,
  },
  {
    id: "resources",
    title: msg("Node load"),
    icon: ChartLineIcon,
    columns: 2,
    visible: (a) => a.canSomewhere("nodes.view"),
    Component: Resources,
  },
  { id: "pinned", title: msg("Pinned servers"), icon: PushPinIcon, columns: 1, visible: seesServers, Component: Pinned },
  { id: "networks", title: msg("Networks"), icon: GraphIcon, columns: 2, visible: (a) => a.can("networks.view"), Component: Networks },
  { id: "top-servers", title: msg("Most players"), icon: UsersThreeIcon, columns: 1, visible: seesServers, Component: TopServers },
  {
    id: "activity",
    title: msg("Recent activity"),
    icon: ScrollIcon,
    columns: 2,
    visible: (a) => a.canSomewhere("logs.view"),
    Component: RecentActivity,
  },
  {
    id: "schedules",
    title: msg("Schedules"),
    icon: CalendarCheckIcon,
    columns: 1,
    visible: (a) => a.can("backupjobs.view") || a.can("policies.view"),
    Component: Schedules,
  },
  {
    id: "actions",
    title: msg("Quick actions"),
    icon: LightningIcon,
    columns: 1,
    visible: (a) => a.canSomewhere("servers.create") || a.can("networks.manage") || a.can("nodes.enroll"),
    Component: QuickActions,
  },
]

export const widgetOf = (id: string) => widgets.find((w) => w.id === id)

/**
 * The widgets the user may see, in the order and widths they chose. Widgets they never
 * arranged, e.g. new ones, come last in their default width.
 */
export function arrange(stored: Widget[], available: WidgetDef[]): Widget[] {
  const kept = stored.filter((w) => available.some((d) => d.id === w.id))
  const added = available.filter((d) => !kept.some((w) => w.id === d.id)).map(({ id, columns }) => ({ id, columns }))
  return [...kept, ...added]
}
