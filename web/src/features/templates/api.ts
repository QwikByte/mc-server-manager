import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import type { Project } from "@/features/plugins/api"
import type { RestartPolicy } from "@/features/servers/api"
import { defaults } from "@/features/servers/server-types"
import { api } from "@/lib/api"

/** What a template sets up for new servers; name, port and storage are chosen per server. */
export interface TemplateSettings {
  type: string
  /** "LATEST" follows new releases. */
  version: string
  memoryMb: number
  java: string
  restartPolicy: RestartPolicy
  aikarFlags: boolean
  jvmOptions: string[]
  cpuLimit: number
  /** Seconds the servers get to stop, e.g. to save their worlds, before they are killed. */
  stopTimeout: number
  /** IANA time zone such as Europe/Berlin; empty for UTC. */
  timeZone: string
  /** Written to server.properties before the first start. */
  properties: Record<string, string>
}

export interface Template extends TemplateSettings {
  id: string
  name: string
  description: string
  /** Projects of Modrinth or Hangar; each new server gets the newest release that suits it. */
  plugins: Project[]
  createdAt: string
}

export interface TemplateInput extends TemplateSettings {
  name: string
  description: string
  /** IDs of projects of Modrinth or Hangar. */
  plugins: string[]
}

/** A template being edited; plugins are kept with their details to show them. */
export type TemplateDraft = Omit<TemplateInput, "plugins"> & { plugins: Project[] }

export const emptyTemplate: TemplateDraft = {
  name: "",
  description: "",
  type: "paper",
  version: "LATEST",
  memoryMb: defaults("paper").memoryMb,
  java: "",
  restartPolicy: "always",
  aikarFlags: false,
  jvmOptions: [],
  cpuLimit: 0,
  stopTimeout: 60,
  timeZone: "",
  properties: {},
  plugins: [],
}

export function draftOf(t: Template): TemplateDraft {
  const { name, description, type, version, memoryMb, java, restartPolicy, aikarFlags, jvmOptions, cpuLimit, properties, plugins } = t
  const settings = { type, version, memoryMb, java, restartPolicy, aikarFlags, jvmOptions, cpuLimit, properties }
  return { name, description, ...settings, stopTimeout: t.stopTimeout, timeZone: t.timeZone, plugins }
}

export const templatesQuery = queryOptions({
  queryKey: ["templates"],
  queryFn: () => api<Template[]>("/templates"),
})

export const templateQuery = (id: string) =>
  queryOptions({
    queryKey: ["templates", id],
    queryFn: () => api<Template>(`/templates/${id}`),
  })

/** Creates a template, or changes it if an ID is given. */
export function useSaveTemplate(id?: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: TemplateInput) =>
      id ? api<Template>(`/templates/${id}`, { method: "PUT", body: input }) : api<Template>("/templates", { body: input }),
    onSuccess: (template) => {
      queryClient.setQueryData(templateQuery(template.id).queryKey, template)
      return queryClient.invalidateQueries({ queryKey: templatesQuery.queryKey, exact: true })
    },
  })
}

export function useDeleteTemplate() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api(`/templates/${id}`, { method: "DELETE" }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: templatesQuery.queryKey }),
  })
}

/** Shows properties one per line as key=value; line breaks in values become \n. */
export function propertiesText(properties: Record<string, string>): string {
  return Object.keys(properties)
    .sort()
    .map((key) => `${key}=${properties[key].replaceAll("\n", "\\n")}`)
    .join("\n")
}

/** Reads properties entered one per line as key=value; empty lines and # comments are skipped. */
export function parseProperties(text: string): Record<string, string> {
  const properties: Record<string, string> = {}
  for (const line of text.split("\n")) {
    const trimmed = line.trim()
    const eq = trimmed.indexOf("=")
    if (!trimmed || trimmed.startsWith("#")) continue
    if (eq < 0) properties[trimmed] = ""
    else properties[trimmed.slice(0, eq).trim()] = trimmed.slice(eq + 1).replaceAll("\\n", "\n")
  }
  return properties
}
