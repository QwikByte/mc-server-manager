import { BugIcon, type Icon, InfoIcon, WarningIcon, WarningOctagonIcon } from "@phosphor-icons/react"
import type { Tone } from "@/components/tone"
import type { Level } from "./api"

/** How levels look; colour always comes with an icon and a label. */
export const levels: Record<Level, { label: string; plural: string; tone: Tone; icon: Icon }> = {
  debug: { label: "Debug", plural: "Debug", tone: "neutral", icon: BugIcon },
  info: { label: "Info", plural: "Info", tone: "info", icon: InfoIcon },
  warn: { label: "Warning", plural: "Warnings", tone: "warning", icon: WarningIcon },
  error: { label: "Error", plural: "Errors", tone: "destructive", icon: WarningOctagonIcon },
}

/** The categories of entries, as the master's logging package names them. */
export const categories: Record<string, string> = {
  auth: "Sign-in",
  users: "Users and groups",
  settings: "Settings",
  nodes: "Nodes",
  servers: "Servers",
  console: "Console",
  files: "Files and configuration",
  plugins: "Plugins and mods",
  backups: "Backups",
  networks: "Networks",
  templates: "Templates",
  policies: "Policies",
  terminal: "Terminal",
  system: "System",
}

export const categoryLabel = (category: string) => categories[category] ?? category

/** Formats the time of an entry: the time of day with seconds, and the date if it isn't today. */
export function formatEntryTime(iso: string) {
  const date = new Date(iso)
  const time = date.toLocaleTimeString(undefined, { timeStyle: "medium" })
  return date.toDateString() === new Date().toDateString()
    ? time
    : `${date.toLocaleDateString(undefined, { dateStyle: "medium" })}, ${time}`
}
