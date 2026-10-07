import type { QueryClient } from "@tanstack/react-query"
import { backupsQuery } from "@/features/backups/api"
import { datastoresQuery } from "@/features/datastores/api"
import { nodesQuery } from "@/features/nodes/api"
import { allServersQuery } from "@/features/servers/api"
import { formatDateTime } from "@/lib/format"
import { type Command, type Flag, masterTarget } from "./api"

/** What a word may become, and what that is, e.g. the name of a server. */
export interface Choice {
  value: string
  hint: string
}

/** The word before the cursor, and the commands or flags it may become, or the kind of value it is. */
export interface Completion {
  start: number
  word: string
  choices?: Choice[]
  kind?: string
  /** The arguments before the word by their kinds, e.g. the server whose backup it is. */
  before?: Partial<Record<string, string>>
}

/** Finds what the word before the cursor may become, walking the commands like the master does. */
export function complete(commands: Command[], line: string, cursor: number): Completion | undefined {
  const word = /\S*$/.exec(line.slice(0, cursor))?.[0] ?? ""
  const start = cursor - word.length
  const words = line.slice(0, start).split(/\s+/).filter(Boolean)
  // help takes the path of a command.
  const help = words[0] === "help"
  let list = commands
  let command: Command | undefined
  let flag: Flag | undefined // whose value comes next
  const args: string[] = []
  for (const w of help ? words.slice(1) : words) {
    if (flag) {
      flag = undefined
    } else if (w.startsWith("-")) {
      const name = w.replace(/^--?/, "").split("=")[0]
      const found = command?.flags?.find((f) => (w.startsWith("--") ? f.name : f.shorthand) === name)
      flag = found?.value && !w.includes("=") ? found : undefined
    } else {
      const sub = args.length === 0 ? list.find((c) => c.name === w) : undefined
      if (sub) [command, list] = [sub, sub.commands ?? []]
      else args.push(w)
    }
  }

  if (flag) return { start, word, kind: flag.value }
  if (word.startsWith("-")) {
    const flags = (!help && command?.flags) || []
    return { start, word, choices: flags.map((f) => ({ value: `--${f.name}`, hint: f.usage })) }
  }
  if (args.length === 0 && list.length > 0) {
    return { start, word, choices: list.map((c) => ({ value: c.name, hint: c.short })) }
  }
  const params = help ? [] : (command?.args ?? [])
  const last = params.at(-1)
  const param = params[args.length] ?? (last?.repeated ? last : undefined)
  if (!param) return undefined
  const before = Object.fromEntries(params.slice(0, args.length).map((p, i) => [p.kind, args[i]]))
  return { start, word, kind: param.kind, before }
}

/** The choices that start with the word; values also if what they are does, e.g. the name of a server. */
export function matching({ word, kind }: Completion, choices: Choice[]) {
  const lower = word.toLowerCase()
  return choices.filter((c) => c.value.startsWith(word) || (kind && c.hint.toLowerCase().startsWith(lower)))
}

/** Replaces the text between start and end with a value, followed by a space unless it is only a part of one. */
export function insert(line: string, start: number, end: number, value: string, part = false) {
  const rest = line.slice(end)
  const text = part || rest.startsWith(" ") ? value : `${value} `
  return { line: line.slice(0, start) + text + rest, cursor: start + text.length }
}

/** The longest start that all values share. */
export function commonStart(values: string[]) {
  let start = values[0] ?? ""
  for (const value of values) while (!value.startsWith(start)) start = start.slice(0, -1)
  return start
}

const staleTime = 10_000

/** The values of a kind that the user may see on the target, e.g. its servers with their names. */
export async function lookup(
  queryClient: QueryClient,
  target: string,
  { kind, before = {} }: Completion,
): Promise<Choice[]> {
  const node = target === masterTarget ? undefined : target
  const servers = async () =>
    (await queryClient.fetchQuery({ ...allServersQuery, staleTime })).filter((s) => !node || s.nodeId === node)
  const datastores = async () =>
    (await queryClient.fetchQuery({ ...datastoresQuery, staleTime })).filter((d) => !node || d.nodeId === node)
  switch (kind) {
    case "server":
      return (await servers()).map((s) => ({ value: s.id, hint: node ? s.name : `${s.name} · ${s.nodeName}` }))
    case "node":
      return (await queryClient.fetchQuery({ ...nodesQuery, staleTime })).map((n) => ({ value: n.id, hint: n.name }))
    case "datastore":
      return (await datastores()).map((d) => ({ value: d.id, hint: d.name }))
    case "database": {
      const datastore = (await datastores()).find((d) => d.id === before.datastore)
      return datastore?.databases.map((d) => ({ value: d.name, hint: "" })) ?? []
    }
    case "backup": {
      // Only of a server the user sees, which also keeps what was typed out of the path of the request.
      const server = node && (await servers()).find((s) => s.id === before.server)
      if (!server) return []
      const backups = await queryClient.fetchQuery({ ...backupsQuery(server.nodeId, server.id), staleTime })
      return backups.map((b) => ({ value: b.id, hint: b.label || formatDateTime(b.createdAt) }))
    }
  }
  return []
}
