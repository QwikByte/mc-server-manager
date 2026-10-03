import { HardDrivesIcon, SlidersHorizontalIcon, TerminalWindowIcon, UserCircleIcon, UsersThreeIcon } from "@phosphor-icons/react"
import type { Access } from "@/features/access/use-access"
import { msg } from "@/lib/i18n"

/** The tabs of the settings and who may see them. */
export const settingsTabs = [
  { to: "/settings", label: msg("General"), icon: SlidersHorizontalIcon, exact: true, visible: (a: Access) => a.can("settings.view") },
  {
    to: "/settings/agents",
    label: msg("Agents"),
    icon: HardDrivesIcon,
    exact: false,
    visible: (a: Access) => a.canSomewhere("nodes.view"),
  },
  {
    to: "/settings/terminal",
    label: msg("Terminal"),
    icon: TerminalWindowIcon,
    exact: false,
    visible: (a: Access) => a.can("terminal.use"),
  },
  { to: "/settings/users", label: msg("Users"), icon: UserCircleIcon, exact: false, visible: (a: Access) => a.can("users.view") },
  { to: "/settings/groups", label: msg("Groups"), icon: UsersThreeIcon, exact: false, visible: (a: Access) => a.can("users.view") },
] as const

/** Whether the user may see any part of the settings. */
export const seesSettings = (access: Access) => settingsTabs.some((t) => t.visible(access))
