/** The properties of server.properties that decide the world of a new game server, with Minecraft's defaults. */
export const worldDefaults: Record<string, string> = { gamemode: "survival", difficulty: "easy", "level-type": "minecraft:normal", "level-seed": "" }

export type World = Record<string, string>

/** The world a new server starts with: the one of its template, or else Minecraft's defaults. */
export const worldOf = (properties: Record<string, string> = {}): World =>
  Object.fromEntries(Object.entries(worldDefaults).map(([key, value]) => [key, properties[key] ?? value]))

/** The properties of the options that were changed; the others stay as the template or Minecraft has them. */
export function worldChanges(world: World, properties?: Record<string, string>) {
  const start = worldOf(properties)
  return Object.fromEntries(Object.entries(world).flatMap(([key, value]) => (value.trim() === start[key] ? [] : [[key, value.trim()]])))
}
