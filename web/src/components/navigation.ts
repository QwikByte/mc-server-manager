import {
  ArchiveIcon,
  CalendarCheckIcon,
  CubeIcon,
  GearSixIcon,
  GraphIcon,
  HardDrivesIcon,
  PuzzlePieceIcon,
  ScrollIcon,
  SquaresFourIcon,
  StackIcon,
} from "@phosphor-icons/react"
import type { Access } from "@/features/access/use-access"
import { seesSettings } from "@/features/settings/tabs"
import { msg } from "@/lib/i18n"

/** The sections of the panel and who may see them. */
export const navigation = [
  {
    title: msg("Manage"),
    links: [
      {
        to: "/",
        label: msg("Overview"),
        icon: SquaresFourIcon,
        visible: (a: Access) => a.canSomewhere("servers.view") || a.canSomewhere("nodes.view"),
      },
      {
        to: "/nodes",
        label: msg("Nodes"),
        icon: HardDrivesIcon,
        visible: (a: Access) => a.canSomewhere("nodes.view") || a.canSomewhere("servers.view"),
      },
      { to: "/servers", label: msg("Servers"), icon: CubeIcon, visible: (a: Access) => a.canSomewhere("servers.view") },
      { to: "/networks", label: msg("Networks"), icon: GraphIcon, visible: (a: Access) => a.can("networks.view") },
      { to: "/templates", label: msg("Templates"), icon: StackIcon, visible: (a: Access) => a.can("templates.view") },
      { to: "/plugins", label: msg("Plugins"), icon: PuzzlePieceIcon, visible: (a: Access) => a.canSomewhere("plugins.manage") },
      { to: "/backups", label: msg("Backups"), icon: ArchiveIcon, visible: (a: Access) => a.can("backupjobs.view") },
      { to: "/policies", label: msg("Policies"), icon: CalendarCheckIcon, visible: (a: Access) => a.can("policies.view") },
    ],
  },
  {
    title: msg("System"),
    links: [
      { to: "/logs", label: msg("Logs"), icon: ScrollIcon, visible: (a: Access) => a.canSomewhere("logs.view") },
      { to: "/settings", label: msg("Settings"), icon: GearSixIcon, visible: seesSettings },
    ],
  },
] as const

/** The first section the user may see, where the panel opens. */
export function home(access: Access) {
  for (const group of navigation) {
    for (const link of group.links) if (link.visible(access)) return link.to
  }
  return "/settings"
}
