import { t } from "i18next"
import { formatBytes, formatElapsed } from "@/lib/format"
import type { Operation } from "./api"

/** What an operation does, e.g. "Create Lobby". name is the name of its server, for operations that don't name it. */
export function titleOf(op: Operation, name?: string): string {
  // While the name of the server loads.
  const subject = op.subject || name || "…"
  const count = Number(op.subject) || 0
  switch (op.kind) {
    case "server.create":
      return t("Create {{name}}", { name: subject })
    case "server.duplicate":
      return t("Create {{name}} as a copy", { name: subject })
    case "server.settings":
      return t("Save the settings of {{name}}", { name: subject })
    case "server.image":
      return t("Update the image of {{name}}", { name: subject })
    case "server.modpack":
      return t("Change the modpack version of {{name}}", { name: subject })
    case "server.stop":
      return t("Stop {{name}}", { name: subject })
    case "server.restart":
      return t("Restart {{name}}", { name: subject })
    case "server.move":
      return t("Move {{name}}", { name: subject })
    case "plugins.install":
      return op.serverId && name
        ? t("Install plugins on {{name}}", { name })
        : t("Install plugins on {{count}} servers", { count, defaultValue_one: "Install plugins on {{count}} server" })
    case "plugins.update":
      if (op.serverId && name) return t("Update the plugins of {{name}}", { name })
      return count
        ? t("Update the plugins of {{count}} servers", { count, defaultValue_one: "Update the plugins of {{count}} server" })
        : t("Update {{name}} everywhere", { name: subject })
    case "plugins.remove":
      return t("Remove {{name}} everywhere", { name: subject })
    case "backup.create":
      return t("Back up {{name}}", { name: subject })
    case "backup.restore":
      return t("Restore a backup of {{name}}", { name: subject })
    case "backup.restore-into":
      return t("Restore a backup into {{name}}", { name: subject })
    case "backup.restore-copy":
      return t("Restore a copy of a backup into {{name}}", { name: subject })
    case "network.create":
      return t("Create the network {{name}}", { name: subject })
    case "network.update":
      return t("Save the network {{name}}", { name: subject })
    case "network.proxy":
      return t("Change the proxy of {{name}}", { name: subject })
    case "network.apply":
      return t("Apply the network {{name}} again", { name: subject })
    case "network.delete":
      return t("Delete the network {{name}}", { name: subject })
    case "network.start":
      return t("Start the network {{name}}", { name: subject })
    case "network.stop":
      return t("Stop the network {{name}}", { name: subject })
    case "network.restart":
      return t("Restart the network {{name}}", { name: subject })
    case "network.rolling-restart":
      return t("Restart {{name}} server by server", { name: subject })
    case "network.safe-restart":
      return t("Restart {{name}} safely", { name: subject })
    case "network.maintenance-on":
      return t("Turn on maintenance of {{name}}", { name: subject })
    case "network.maintenance-off":
      return t("Turn off maintenance of {{name}}", { name: subject })
    case "network.maintenance-timer":
      return t("Plan maintenance of {{name}}", { name: subject })
    case "network.maintenance-end":
      return t("Plan the end of maintenance of {{name}}", { name: subject })
    case "overlay.leave":
      return t("Remove {{name}} from the private network", { name: subject })
    case "overlay.rotate":
      return t("Rotate the key of {{name}}", { name: subject })
    case "node.delete":
      return t("Remove {{name}}", { name: subject })
    case "players.kick":
      return t("Kick {{name}}", { name: subject })
    case "players.ban":
      return t("Ban {{name}}", { name: subject })
    case "players.pardon":
      return t("Pardon {{name}}", { name: subject })
    case "players.whitelist_add":
      return t("Add {{name}} to the whitelist", { name: subject })
    case "players.whitelist_remove":
      return t("Remove {{name}} from the whitelist", { name: subject })
    case "players.op":
      return t("Make {{name}} an operator", { name: subject })
    case "players.deop":
      return t("Revoke operator status from {{name}}", { name: subject })
    case "players.whitelist_on":
      return t("Turn on the whitelist of {{count}} servers", { count, defaultValue_one: "Turn on the whitelist of {{count}} server" })
    case "players.whitelist_off":
      return t("Turn off the whitelist of {{count}} servers", { count, defaultValue_one: "Turn off the whitelist of {{count}} server" })
    case "fileset.apply":
      return t("Apply the file set {{name}}", { name: subject })
    case "files.extract":
      return t("Extract {{name}}", { name: subject })
    case "files.copy":
      return t("Copy {{name}}", { name: subject })
    case "datastore.create":
      return t("Create the datastore {{name}}", { name: subject })
    case "datastore.update":
      return t("Change the datastore {{name}}", { name: subject })
    case "datastore.backup":
      return t("Back up the datastore {{name}}", { name: subject })
    case "datastore.restore":
      return t("Restore a backup of the datastore {{name}}", { name: subject })
    case "servers.start":
      return t("Start {{count}} servers", { count, defaultValue_one: "Start {{count}} server" })
    case "servers.stop":
      return t("Stop {{count}} servers", { count, defaultValue_one: "Stop {{count}} server" })
    case "servers.restart":
      return t("Restart {{count}} servers", { count, defaultValue_one: "Restart {{count}} server" })
    case "servers.command":
      return t("Send a command to {{count}} servers", { count, defaultValue_one: "Send a command to {{count}} server" })
  }
  return op.kind
}

/** What a step of an operation does. */
export function stepOf(op: Operation, step: string): string {
  const verb = op.kind.split(".")[1]
  if (op.kind.startsWith("datastore.")) {
    switch (step) {
      case "image":
        return t("Download the database image")
      case "start":
        return t("Start the datastore")
      case "network":
        return t("Connect the servers of the network")
      case "dump":
        return t("Dump the databases")
      case "load":
        return t("Load the dumps")
      case "check":
        return t("Check the backup")
    }
  }
  switch (step) {
    case "image":
      return t("Prepare the server image")
    case "container":
      return t("Create the container")
    case "plugins":
      if (op.kind === "plugins.update") return t("Update the plugins")
      if (op.kind === "plugins.remove") return t("Remove the plugin")
      return t("Install the plugins")
    case "modpack":
      return t("Download the modpack")
    case "mods":
      return op.kind === "server.modpack" ? t("Change the mods and files of the modpack") : t("Install the mods of the modpack")
    case "save":
      return t("Save the worlds")
    case "copy":
    case "copying":
      if (op.kind === "backup.restore-copy") return t("Fetch the copy")
      return op.kind === "backup.restore-into" ? t("Copy the backup") : t("Copy the data")
    case "archive":
      return op.kind.startsWith("backup.restore") ? t("Back up what is replaced") : t("Pack the backup")
    case "restore":
      return t("Unpack the backup")
    case "extract":
      return t("Extract the files")
    case "stop":
    case "stopping":
      return t("Stop the server")
    case "start":
      return t("Start the server")
    case "backups":
      return t("Copy the backups")
    case "finishing":
      return t("Set the server up on the new node")
    case "proxy":
      return t("Configure the proxy")
    case "proxy-stop":
      return t("Stop the proxy")
    case "proxy-start":
      return t("Start the proxy")
    case "proxy-restart":
      return t("Restart the proxy")
    case "settings":
      return t("Take over the settings of the old proxy")
    case "old-proxy":
      return t("Take the old proxy out of the network")
    case "plugin":
      return t("Install the Maintenance plugin")
    case "bedrock":
      return t("Install Geyser and Floodgate")
    case "bedrock-remove":
      return t("Remove Geyser and Floodgate")
    case "maintenance":
      if (op.kind === "network.maintenance-timer" || op.kind === "network.maintenance-end") return t("Start the timer of the Maintenance plugin")
      return op.kind === "network.maintenance-off" ? t("Turn maintenance off") : t("Turn maintenance on")
    case "overlay":
      return t("Leave the private network")
    case "key":
      return t("Create a new key and give it to the other nodes")
    case "networks":
      return t("Configure the networks again")
    case "node":
      return t("Remove the node")
    case "files":
      return t("Write the files on the servers")
    case "restart":
      if (op.kind.startsWith("plugins.")) return t("Restart the servers whose plugins changed")
      return op.kind === "server.restart" ? t("Restart the server") : t("Restart the servers whose files changed")
    case "warn":
      return t("Warn the players")
    case "servers":
      if (op.kind.startsWith("players.")) return t("Apply it on the servers")
      if (verb === "rolling-restart" || verb === "safe-restart" || op.kind.startsWith("plugins.")) return t("Restart the servers one after the other")
      if (verb === "start") return t("Start the servers")
      if (verb === "stop") return t("Stop the servers")
      if (verb === "restart") return t("Restart the servers")
      if (verb === "command") return t("Send the command")
      if (op.kind === "network.delete") return t("Make the servers standalone")
      return t("Configure the servers")
  }
  return step
}

/** How far the current step of an operation is, e.g. "245 MB of 610 MB", if it tells. */
export function amountOf(op: Operation): string | undefined {
  if (op.unit === "bytes" && op.total > 0) return t("{{done}} of {{total}}", { done: formatBytes(op.done), total: formatBytes(op.total) })
  if (op.unit === "bytes" && op.done > 0) return formatBytes(op.done)
  if (op.unit === "servers" && op.total > 1) return t("{{done}} of {{count}} servers", { done: op.done, count: op.total })
  if (op.unit === "backups" && op.total > 0) return t("{{done}} of {{count}} backups", { done: op.done, count: op.total })
  if (op.unit === "files" && op.total > 0) return t("{{done}} of {{count}} files", { done: op.done, count: op.total })
  if (op.unit === "minutes" && op.total > 0) return t("{{time}} left", { time: formatElapsed(warningLeft(op)) })
  return undefined
}

/** How much of the current step of an operation is done, from 0 to 1, if it is known. */
export function shareOf(op: Operation): number | undefined {
  if (op.unit === "minutes" && op.total > 0) return 1 - warningLeft(op) / (op.total * 60_000)
  if (op.total > 0 && (op.unit !== "servers" || op.total > 1)) return Math.min(op.done / op.total, 1)
  return undefined
}

/** The milliseconds left of a warning, which counts down the minutes in total from the start of its operation. */
const warningLeft = (op: Operation) => Math.max(0, Date.parse(op.startedAt) + op.total * 60_000 - Date.now())
