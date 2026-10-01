export type Kind =
  | { type: "boolean" }
  | { type: "number"; min?: number; max?: number }
  | { type: "select"; options: [value: string, label: string][] }
  | { type: "text"; mono?: boolean }
  | { type: "motd" }

export const groups = ["Gameplay", "World", "Players", "Server list", "Resource pack", "Performance", "Advanced"] as const
export type Group = (typeof groups)[number]

export interface Definition {
  label: string
  description: string
  group: Group
  kind: Kind
}

const bool = { type: "boolean" } as const
const num = (min?: number, max?: number) => ({ type: "number", min, max }) as const
const permissionLevels: [string, string][] = [
  ["1", "1 – bypass spawn protection"],
  ["2", "2 – cheats and command blocks"],
  ["3", "3 – manage players"],
  ["4", "4 – all commands"],
]

/**
 * Known properties of server.properties. Only those in the file are shown, because
 * Minecraft versions differ in which ones they have; others appear under Advanced.
 */
export const definitions: Record<string, Definition> = {
  gamemode: {
    label: "Game mode",
    description: "Game mode of players who join for the first time.",
    group: "Gameplay",
    kind: {
      type: "select",
      options: [
        ["survival", "Survival"],
        ["creative", "Creative"],
        ["adventure", "Adventure"],
        ["spectator", "Spectator"],
      ],
    },
  },
  difficulty: {
    label: "Difficulty",
    description: "How dangerous the world is.",
    group: "Gameplay",
    kind: {
      type: "select",
      options: [
        ["peaceful", "Peaceful"],
        ["easy", "Easy"],
        ["normal", "Normal"],
        ["hard", "Hard"],
      ],
    },
  },
  "force-gamemode": {
    label: "Force game mode",
    description: "Puts players into the game mode above every time they join.",
    group: "Gameplay",
    kind: bool,
  },
  hardcore: {
    label: "Hardcore",
    description: "Players become spectators when they die, and the difficulty stays on hard.",
    group: "Gameplay",
    kind: bool,
  },
  pvp: { label: "PvP", description: "Players can hurt each other.", group: "Gameplay", kind: bool },
  "allow-flight": {
    label: "Allow flight",
    description: "Lets players fly in survival, e.g. with plugins or mods. Otherwise flying players are kicked.",
    group: "Gameplay",
    kind: bool,
  },
  "allow-nether": { label: "Nether", description: "Players can travel to the Nether.", group: "Gameplay", kind: bool },
  "enable-command-block": { label: "Command blocks", description: "Command blocks run their commands.", group: "Gameplay", kind: bool },
  "spawn-protection": {
    label: "Spawn protection",
    description: "Radius in blocks around the world spawn that only operators can change. 0 turns it off.",
    group: "Gameplay",
    kind: num(0),
  },
  "spawn-monsters": { label: "Monsters", description: "Monsters spawn.", group: "Gameplay", kind: bool },
  "spawn-animals": { label: "Animals", description: "Animals spawn.", group: "Gameplay", kind: bool },
  "spawn-npcs": { label: "Villagers", description: "Villagers spawn.", group: "Gameplay", kind: bool },

  "level-name": {
    label: "World folder",
    description: "Folder of the world. A new name creates a new world on the next start.",
    group: "World",
    kind: { type: "text", mono: true },
  },
  "level-seed": {
    label: "Seed",
    description: "Seed of a newly generated world. Empty picks a random one.",
    group: "World",
    kind: { type: "text", mono: true },
  },
  "level-type": {
    label: "World type",
    description: "Type of a newly generated world.",
    group: "World",
    kind: {
      type: "select",
      options: [
        ["minecraft:normal", "Default"],
        ["minecraft:flat", "Superflat"],
        ["minecraft:large_biomes", "Large biomes"],
        ["minecraft:amplified", "Amplified"],
        ["minecraft:single_biome_surface", "Single biome"],
      ],
    },
  },
  "generate-structures": {
    label: "Structures",
    description: "New chunks get villages, temples and other structures.",
    group: "World",
    kind: bool,
  },
  "generator-settings": {
    label: "Generator settings",
    description: "Settings of superflat worlds as JSON.",
    group: "World",
    kind: { type: "text", mono: true },
  },
  "initial-enabled-packs": {
    label: "Enabled data packs",
    description: "Data packs turned on in a newly generated world, separated by commas.",
    group: "World",
    kind: { type: "text", mono: true },
  },
  "initial-disabled-packs": {
    label: "Disabled data packs",
    description: "Data packs turned off in a newly generated world, separated by commas.",
    group: "World",
    kind: { type: "text", mono: true },
  },
  "max-world-size": { label: "World size", description: "Radius of the world border in blocks.", group: "World", kind: num(1, 29999984) },
  "view-distance": {
    label: "View distance",
    description: "Chunks around players that are sent to them.",
    group: "World",
    kind: num(3, 32),
  },
  "simulation-distance": {
    label: "Simulation distance",
    description: "Chunks around players that are updated, so that crops grow and mobs move.",
    group: "World",
    kind: num(3, 32),
  },

  "max-players": { label: "Player slots", description: "Players who can be online at the same time.", group: "Players", kind: num(0) },
  "white-list": {
    label: "Whitelist",
    description: "Only players on the whitelist can join. Add them in the console with whitelist add <name>.",
    group: "Players",
    kind: bool,
  },
  "enforce-whitelist": {
    label: "Enforce whitelist",
    description: "Kicks online players who aren't on the whitelist when it is reloaded.",
    group: "Players",
    kind: bool,
  },
  "online-mode": {
    label: "Online mode",
    description: "Checks with Mojang that players own their account. Turned off, anyone can join with any name.",
    group: "Players",
    kind: bool,
  },
  "op-permission-level": {
    label: "Operator permissions",
    description: "What operators are allowed to do.",
    group: "Players",
    kind: { type: "select", options: permissionLevels },
  },
  "function-permission-level": {
    label: "Function permissions",
    description: "Permission level of functions in data packs.",
    group: "Players",
    kind: { type: "select", options: permissionLevels },
  },
  "player-idle-timeout": {
    label: "Idle timeout",
    description: "Kicks players who are idle for this many minutes. 0 turns it off.",
    group: "Players",
    kind: num(0),
  },
  "enforce-secure-profile": {
    label: "Signed chat",
    description: "Players need a profile key signed by Mojang to chat.",
    group: "Players",
    kind: bool,
  },
  "prevent-proxy-connections": {
    label: "Block VPNs",
    description: "Kicks players whose connection doesn't come from the network they signed in from.",
    group: "Players",
    kind: bool,
  },
  "enable-code-of-conduct": {
    label: "Code of conduct",
    description: "Shows players the code of conduct from the codeofconduct folder when they join.",
    group: "Players",
    kind: bool,
  },
  "log-ips": { label: "Log IP addresses", description: "Writes the IP addresses of players to the log.", group: "Players", kind: bool },

  motd: {
    label: "Message of the day",
    description: "Shown below the server name in the server list, in up to two lines.",
    group: "Server list",
    kind: { type: "motd" },
  },
  "enable-status": {
    label: "Show as online",
    description: "The server answers status requests of the server list.",
    group: "Server list",
    kind: bool,
  },
  "hide-online-players": {
    label: "Hide player names",
    description: "The server list doesn't show who is online.",
    group: "Server list",
    kind: bool,
  },

  "resource-pack": {
    label: "URL",
    description: "Resource pack offered to players when they join.",
    group: "Resource pack",
    kind: { type: "text", mono: true },
  },
  "resource-pack-sha1": {
    label: "SHA-1",
    description: "Hash of the pack, so that players notice when it changed.",
    group: "Resource pack",
    kind: { type: "text", mono: true },
  },
  "resource-pack-id": { label: "ID", description: "UUID of the pack.", group: "Resource pack", kind: { type: "text", mono: true } },
  "require-resource-pack": {
    label: "Required",
    description: "Players who decline the pack are disconnected.",
    group: "Resource pack",
    kind: bool,
  },
  "resource-pack-prompt": {
    label: "Prompt",
    description: "Message shown when the pack is offered.",
    group: "Resource pack",
    kind: { type: "text" },
  },

  "network-compression-threshold": {
    label: "Compression threshold",
    description: "Packets bigger than this many bytes are compressed. -1 turns compression off.",
    group: "Performance",
    kind: num(-1),
  },
  "max-tick-time": {
    label: "Watchdog",
    description: "Stops the server when a tick takes longer than this many milliseconds. -1 turns it off.",
    group: "Performance",
    kind: num(-1),
  },
  "sync-chunk-writes": {
    label: "Synchronous chunk writes",
    description: "Safer against data loss, but slower.",
    group: "Performance",
    kind: bool,
  },
  "entity-broadcast-range-percentage": {
    label: "Entity range",
    description: "How far away players see entities, in percent of the default.",
    group: "Performance",
    kind: num(10, 1000),
  },
  "rate-limit": {
    label: "Packet limit",
    description: "Kicks players who send more packets per second. 0 turns it off.",
    group: "Performance",
    kind: num(0),
  },
  "pause-when-empty-seconds": {
    label: "Pause when empty",
    description: "Pauses the server after it was empty for this many seconds. 0 or less never pauses.",
    group: "Performance",
    kind: num(),
  },
  "region-file-compression": {
    label: "Region compression",
    description: "How the world's region files are compressed.",
    group: "Performance",
    kind: {
      type: "select",
      options: [
        ["deflate", "Deflate"],
        ["lz4", "LZ4 (faster, larger)"],
        ["none", "None"],
      ],
    },
  },
  "use-native-transport": {
    label: "Native transport",
    description: "Uses the faster network transport of Linux.",
    group: "Performance",
    kind: bool,
  },

  "broadcast-rcon-to-ops": {
    label: "Show console commands to operators",
    description: "Operators see the output of commands from the panel's console, which uses RCON.",
    group: "Advanced",
    kind: bool,
  },
  "broadcast-console-to-ops": {
    label: "Show server console to operators",
    description: "Operators see the output of commands typed in the server console.",
    group: "Advanced",
    kind: bool,
  },
  "enable-query": { label: "Query", description: "Answers GameSpy4 queries for server information.", group: "Advanced", kind: bool },
  "query.port": { label: "Query port", description: "Port for queries inside the container.", group: "Advanced", kind: num(1, 65535) },
  "management-server-enabled": {
    label: "Management API",
    description: "Turns on Minecraft's management API (JSON-RPC over WebSocket) inside the container.",
    group: "Advanced",
    kind: bool,
  },
  "bug-report-link": {
    label: "Bug report link",
    description: "Link players see to report problems with the server.",
    group: "Advanced",
    kind: { type: "text", mono: true },
  },
  debug: { label: "Debug logging", description: "Writes more details to the log.", group: "Advanced", kind: bool },
  "accepts-transfers": {
    label: "Accept transfers",
    description: "Accepts players sent over by other servers.",
    group: "Advanced",
    kind: bool,
  },
}

/** The definition of a property. Unknown ones go under Advanced, as a switch if they look boolean. */
export function definition(key: string, value?: string): Definition {
  const kind: Kind = value === "true" || value === "false" ? { type: "boolean" } : { type: "text", mono: true }
  return definitions[key] ?? { label: key, description: "", group: "Advanced", kind }
}

/** Returns an error message if the value doesn't fit the property. */
export function check(key: string, value: string): string | undefined {
  const kind = definition(key).kind
  if (kind.type !== "number") return undefined
  const n = Number(value)
  if (value.trim() === "" || !Number.isInteger(n)) return "Enter a whole number."
  if (kind.min !== undefined && n < kind.min) return `The minimum is ${kind.min}.`
  if (kind.max !== undefined && n > kind.max) return `The maximum is ${kind.max}.`
}
