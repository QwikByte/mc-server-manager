import { GearSixIcon, HardDrivesIcon, SlidersHorizontalIcon, TerminalWindowIcon, UserCircleIcon, UsersThreeIcon } from "@phosphor-icons/react"
import type { Access } from "@/features/access/use-access"
import { msg } from "@/lib/i18n"

/** The settings: tabs under one header, and who may see them. */
export const settings = {
  label: msg("Settings"),
  icon: GearSixIcon,
  tone: "neutral",
  tabs: [
    { to: "/settings", label: msg("General"), icon: SlidersHorizontalIcon, exact: true, visible: (a: Access) => a.can("settings.view") },
    { to: "/settings/users", label: msg("Users"), icon: UserCircleIcon, visible: (a: Access) => a.can("users.view") },
    { to: "/settings/groups", label: msg("Groups"), icon: UsersThreeIcon, visible: (a: Access) => a.can("users.view") },
    { to: "/settings/agents", label: msg("Agents"), icon: HardDrivesIcon, visible: (a: Access) => a.canSomewhere("nodes.view") },
    { to: "/settings/terminal", label: msg("Terminal"), icon: TerminalWindowIcon, visible: (a: Access) => a.can("terminal.use") },
  ],
} as const

/** Whether the user may see any part of the settings. */
export const seesSettings = (access: Access) => settings.tabs.some((t) => t.visible(access))
