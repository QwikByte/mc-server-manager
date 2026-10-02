import { ArchiveIcon, CalendarCheckIcon, GearSixIcon, GraphIcon, HardDrivesIcon, PuzzlePieceIcon, StackIcon } from "@phosphor-icons/react"
import type { Access } from "@/features/access/use-access"
import { seesSettings } from "@/features/settings/tabs"

/** The sections of the panel and who may see them. */
export const navigation = [
  {
    title: "Manage",
    links: [
      {
        to: "/nodes",
        label: "Nodes",
        icon: HardDrivesIcon,
        visible: (a: Access) => a.canSomewhere("nodes.view") || a.canSomewhere("servers.view"),
      },
      { to: "/networks", label: "Networks", icon: GraphIcon, visible: (a: Access) => a.can("networks.view") },
      { to: "/templates", label: "Templates", icon: StackIcon, visible: (a: Access) => a.can("templates.view") },
      { to: "/plugins", label: "Plugins", icon: PuzzlePieceIcon, visible: (a: Access) => a.canSomewhere("plugins.manage") },
      { to: "/backups", label: "Backups", icon: ArchiveIcon, visible: (a: Access) => a.can("backupjobs.view") },
      { to: "/policies", label: "Policies", icon: CalendarCheckIcon, visible: (a: Access) => a.can("policies.view") },
    ],
  },
  { title: "System", links: [{ to: "/settings", label: "Settings", icon: GearSixIcon, visible: seesSettings }] },
] as const

/** The first section the user may see, where the panel opens. */
export function home(access: Access) {
  for (const group of navigation) {
    for (const link of group.links) if (link.visible(access)) return link.to
  }
  return "/settings"
}
