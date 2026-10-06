import {
  CrownSimpleIcon,
  GavelIcon,
  type Icon,
  ListChecksIcon,
  ListDashesIcon,
  ProhibitIcon,
  SignOutIcon,
  UserCircleMinusIcon,
  UserMinusIcon,
  UserPlusIcon,
} from "@phosphor-icons/react"
import { t } from "i18next"
import type { Permission } from "@/features/access/permissions"
import type { PlayerAction } from "./api"

interface ActionInfo {
  icon: Icon
  /** As a menu item, e.g. "Ban…". */
  label: () => string
  /** What it does to a player, e.g. "Ban Alex". */
  title: (name: string) => string
  /** That it was done, e.g. "Banned Alex". */
  done: (name: string) => string
  /** Kicks and bans tell the player why. */
  reason?: boolean
  destructive?: boolean
}

export const playerActions: Record<PlayerAction, ActionInfo> = {
  kick: {
    icon: SignOutIcon,
    label: () => t("Kick…"),
    title: (name) => t("Kick {{name}}", { name }),
    done: (name) => t("Kicked {{name}}", { name }),
    reason: true,
    destructive: true,
  },
  ban: {
    icon: GavelIcon,
    label: () => t("Ban…"),
    title: (name) => t("Ban {{name}}", { name }),
    done: (name) => t("Banned {{name}}", { name }),
    reason: true,
    destructive: true,
  },
  pardon: {
    icon: ProhibitIcon,
    label: () => t("Pardon…"),
    title: (name) => t("Pardon {{name}}", { name }),
    done: (name) => t("Pardoned {{name}}", { name }),
  },
  whitelist_add: {
    icon: UserPlusIcon,
    label: () => t("Add to the whitelist…"),
    title: (name) => t("Add {{name}} to the whitelist", { name }),
    done: (name) => t("Added {{name}} to the whitelist", { name }),
  },
  whitelist_remove: {
    icon: UserMinusIcon,
    label: () => t("Remove from the whitelist…"),
    title: (name) => t("Remove {{name}} from the whitelist", { name }),
    done: (name) => t("Removed {{name}} from the whitelist", { name }),
  },
  op: {
    icon: CrownSimpleIcon,
    label: () => t("Make operator…"),
    title: (name) => t("Make {{name}} an operator", { name }),
    done: (name) => t("Made {{name}} an operator", { name }),
  },
  deop: {
    icon: UserCircleMinusIcon,
    label: () => t("Revoke operator status…"),
    title: (name) => t("Revoke operator status from {{name}}", { name }),
    done: (name) => t("{{name}} is no longer an operator", { name }),
  },
  whitelist_on: {
    icon: ListChecksIcon,
    label: () => t("Turn the whitelist on…"),
    title: () => t("Turn the whitelist on"),
    done: () => t("Turned the whitelist on"),
  },
  whitelist_off: {
    icon: ListDashesIcon,
    label: () => t("Turn the whitelist off…"),
    title: () => t("Turn the whitelist off"),
    done: () => t("Turned the whitelist off"),
  },
}

/** The permissions an action needs on each server: making operators lets players run any command. */
export const needs = (action: PlayerAction): Permission[] =>
  action === "op" || action === "deop" ? ["players.manage", "console.commands"] : ["players.manage"]
