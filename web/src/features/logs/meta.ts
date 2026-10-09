import { BugIcon, type Icon, InfoIcon, WarningIcon, WarningOctagonIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import type { Tone } from "@/components/tone"
import { dayOf, formatTime } from "@/lib/format"
import { msg } from "@/lib/i18n"
import type { Level } from "./api"

/** How levels look; colour always comes with an icon and a label. The labels are translated with t. */
export const levels: Record<Level, { label: string; plural: string; tone: Tone; icon: Icon }> = {
  debug: { label: msg("Debug"), plural: msg("Debug"), tone: "neutral", icon: BugIcon },
  info: { label: msg("Info"), plural: msg("Info"), tone: "info", icon: InfoIcon },
  warn: { label: msg("Warning"), plural: msg("Warnings"), tone: "warning", icon: WarningIcon },
  error: { label: msg("Error"), plural: msg("Errors"), tone: "destructive", icon: WarningOctagonIcon },
}

/** The categories of entries, as the master's logging package names them. */
export const categories: Record<string, string> = {
  auth: msg("Sign-in"),
  users: msg("Users and groups"),
  settings: msg("Settings"),
  nodes: msg("Nodes"),
  servers: msg("Servers"),
  console: msg("Console"),
  files: msg("Files and configuration"),
  plugins: msg("Plugins and mods"),
  backups: msg("Backups"),
  networks: msg("Networks"),
  databases: msg("Databases"),
  usage: msg("Usage"),
  players: msg("Players"),
  templates: msg("Templates"),
  policies: msg("Schedules"),
  workflows: msg("Workflows"),
  terminal: msg("Terminal"),
  notifications: msg("Notifications"),
  system: msg("System"),
}

export const categoryLabel = (category: string) => (categories[category] ? t(categories[category]) : category)

/** Formats the time of an entry: the time of day with seconds, and the date if it isn't today. */
export function formatEntryTime(iso: string) {
  const time = formatTime(iso, { timeStyle: "medium" })
  return dayOf(iso) === dayOf(Date.now()) ? time : `${formatTime(iso, { dateStyle: "medium" })}, ${time}`
}
