import {
  ArrowCircleUpIcon,
  CaretDownIcon,
  CheckIcon,
  DownloadSimpleIcon,
  MagnifyingGlassIcon,
  PlusIcon,
  PuzzlePieceIcon,
  TrashIcon,
  UploadSimpleIcon,
} from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi } from "@tanstack/react-router"
import { t } from "i18next"
import { useRef, useState } from "react"
import { toast } from "sonner"
import { Callout, ErrorCallout } from "@/components/callout"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { EmptyState } from "@/components/empty-state"
import { Section } from "@/components/section"
import { Segmented } from "@/components/segmented"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import type { ServerRef } from "@/features/networks/api"
import { type Server, useServer } from "@/features/servers/api"
import { serverType } from "@/features/servers/server-types"
import { formatBytes } from "@/lib/format"
import { locale } from "@/lib/i18n"
import { type InstalledPlugin, type Project, type ProjectVersion, pluginsQuery, type SearchHit, useChangePlugins, useInstallPlugins } from "./api"
import { PluginIcon } from "./plugin-icon"
import { PluginSearch } from "./plugin-search"
import { VersionMenu } from "./version-menu"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId/plugins")

type Kind = "plugins" | "mods"
type Show = "all" | "updates" | "foreign"

/** The texts that name what a server loads, plugins or mods. */
function texts(kind: Kind, type: string) {
  return kind === "plugins"
    ? {
        none: t("No plugins yet"),
        add: t("Add plugins"),
        addTitle: t("Add plugins from Modrinth"),
        addDescription: t("Only plugins for {{type}} are shown. What they require is installed too.", { type }),
        updated: (count: number) =>
          t("Updated {{count}} plugins. Restart the server to load them.", {
            count,
            defaultValue_one: "Updated {{count}} plugin. Restart the server to load it.",
          }),
      }
    : {
        none: t("No mods yet"),
        add: t("Add mods"),
        addTitle: t("Add mods from Modrinth"),
        addDescription: t("Only mods for {{type}} are shown. What they require is installed too.", { type }),
        updated: (count: number) =>
          t("Updated {{count}} mods. Restart the server to load them.", {
            count,
            defaultValue_one: "Updated {{count}} mod. Restart the server to load it.",
          }),
      }
}

type Words = ReturnType<typeof texts>

const name = (p: InstalledPlugin) => p.project?.title ?? p.fileName

/** The Plugins tab of a server, Mods for modded servers. */
export function ServerPluginsPage() {
  const { nodeId, serverId } = route.useParams()
  const manage = useAccess().can("plugins.manage", nodeId, serverId)
  const { server } = useServer(nodeId, serverId)
  const ref = { nodeId, serverId }
  const { data, isPending, error } = useQuery(pluginsQuery(ref))
  const [input, setInput] = useState("")
  const [show, setShow] = useState<Show>("all")
  const [sort, setSort] = useState<"name" | "size">("name")

  if (!server || isPending) return <Skeleton className="h-64 rounded-xl" />
  if (error) return <ErrorCallout error={error} />
  const kind = serverType(server.type).addons?.kind ?? "plugins"
  const words = texts(kind, serverType(server.type).label)
  const count = data.plugins.length
  const installed = data.plugins.flatMap((p) => (p.project ? [p.project.id] : []))
  const updates = data.plugins.flatMap((p) => (p.update && p.project ? [p.project] : []))
  const foreign = data.plugins.filter((p) => !p.project).length
  const needle = input.trim().toLowerCase()
  const shown = data.plugins
    .filter((p) => show === "all" || (show === "updates" ? p.update : !p.project))
    .filter((p) => [name(p), p.fileName].some((text) => text.toLowerCase().includes(needle)))
    .sort(sort === "size" ? (a, b) => b.size - a.size : (a, b) => name(a).localeCompare(name(b), locale))

  return (
    <Section
      title={
        kind === "plugins"
          ? t("{{count}} plugins", { count, defaultValue_one: "{{count}} plugin" })
          : t("{{count}} mods", { count, defaultValue_one: "{{count}} mod" })
      }
      className="mt-0"
      actions={
        manage && (
          <div className="flex flex-wrap gap-2">
            {updates.length > 0 && <UpdateAllButton projects={updates} serverRef={ref} words={words} />}
            <UploadButton serverRef={ref} />
            <AddDialog server={server} serverRef={ref} words={words} installed={installed} />
          </div>
        )
      }
    >
      {data.catalogueError && (
        <Callout tone="warning" className="mb-4">
          {t("Modrinth couldn't identify the files: {{error}}", { error: data.catalogueError })}
        </Callout>
      )}
      {count === 0 ? (
        <EmptyState icon={PuzzlePieceIcon} tone="warning" title={words.none}>
          {manage && <AddDialog server={server} serverRef={ref} words={words} installed={installed} />}
        </EmptyState>
      ) : (
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <InputGroup className="min-w-48 flex-1">
              <InputGroupAddon>
                <MagnifyingGlassIcon />
              </InputGroupAddon>
              <InputGroupInput
                type="search"
                placeholder={t("Search by name or file")}
                aria-label={t("Search the installed files")}
                value={input}
                onChange={(e) => setInput(e.target.value)}
              />
            </InputGroup>
            <Segmented
              label={t("Show")}
              value={show}
              onChange={setShow}
              options={[
                { value: "all", label: t("All ({{number}})", { number: count }) },
                { value: "updates", label: t("Updates ({{number}})", { number: updates.length }) },
                { value: "foreign", label: t("Not from Modrinth ({{number}})", { number: foreign }) },
              ]}
            />
            <Select value={sort} onValueChange={(v) => setSort(v as typeof sort)}>
              <SelectTrigger aria-label={t("Sort")} className="max-sm:flex-1">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="name">{t("By name")}</SelectItem>
                <SelectItem value="size">{t("By size")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {shown.length === 0 ? (
            <p className="surface rounded-xl py-8 text-center text-sm text-muted-foreground">{t("Nothing matches the filters.")}</p>
          ) : (
            <ul className="surface divide-y rounded-xl">
              {shown.map((plugin) => (
                <PluginRow key={plugin.fileName} plugin={plugin} server={server} serverRef={ref} manage={manage} />
              ))}
            </ul>
          )}
        </div>
      )}
    </Section>
  )
}

/** Installs a project on a server, the newest suitable release or a chosen version, and tells how it went. */
function useInstallOn(serverRef: ServerRef) {
  const install = useInstallPlugins()
  const run = (project: Project, version?: ProjectVersion) =>
    toast.promise(
      install.mutateAsync({ projects: [project.id], servers: [serverRef], versions: version && { [project.id]: version.id } }).then(failIfAny),
      {
        loading: t("Installing {{name}}…", { name: version ? `${project.title} ${version.number}` : project.title }),
        success: (files) => t("Installed {{files}}. Restart the server to load it.", { files }),
        error: (e: Error) => e.message,
      },
    )
  return { run, pending: install.isPending }
}

function PluginRow({ plugin, server, serverRef, manage }: { plugin: InstalledPlugin; server: Server; serverRef: ServerRef; manage: boolean }) {
  const change = useChangePlugins(serverRef)
  const install = useInstallOn(serverRef)
  const { project, update } = plugin

  return (
    <li className="flex items-center gap-3 px-4 py-3">
      <PluginIcon src={project?.icon} />
      <div className="min-w-0 flex-1">
        <p className="flex items-center gap-2 truncate text-sm font-semibold">
          {project ? (
            <a href={`https://modrinth.com/project/${project.slug}`} target="_blank" rel="noreferrer" className="truncate hover:underline">
              {project.title}
            </a>
          ) : (
            <span className="truncate">{plugin.fileName}</span>
          )}
          {plugin.version && manage && project ? (
            <VersionMenu project={project.id} type={server.type} version={server.version} current={plugin.versionId} onPick={(v) => install.run(project, v)}>
              <Button
                variant="ghost"
                size="xs"
                disabled={install.pending}
                aria-label={t("Change the version of {{name}}", { name: project.title })}
                className="min-w-0 font-mono font-normal text-muted-foreground"
              >
                <span className="truncate">{plugin.version}</span>
                <CaretDownIcon />
              </Button>
            </VersionMenu>
          ) : (
            plugin.version && <span className="truncate font-mono text-xs font-normal text-muted-foreground">{plugin.version}</span>
          )}
        </p>
        <p className="truncate text-xs text-muted-foreground">
          {project ? plugin.fileName : t("Not from Modrinth")} · {formatBytes(plugin.size)}
        </p>
      </div>
      {update && !manage && <Pill tone="info">{t("Update to {{version}}", { version: update })}</Pill>}
      {update && manage && project && (
        <Button size="sm" variant="outline" disabled={install.pending} onClick={() => install.run(project)}>
          <ArrowCircleUpIcon />
          <span className="max-sm:sr-only">{t("Update to {{version}}", { version: update })}</span>
        </Button>
      )}
      {manage && (
        <ConfirmDialog
          trigger={
            <Button
              size="icon-sm"
              variant="ghost"
              aria-label={t("Remove {{name}}", { name: plugin.fileName })}
              title={t("Remove")}
              disabled={change.isPending}
              className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
            >
              <TrashIcon />
            </Button>
          }
          title={t("Remove {{name}}?", { name: project?.title ?? plugin.fileName })}
          description={t("This deletes {{file}}. Its configuration in the server's folder is kept.", { file: plugin.fileName })}
          action={t("Remove")}
          destructive
          onConfirm={() =>
            change.mutate(
              { action: "remove", fileName: plugin.fileName },
              { onSuccess: () => toast.success(t("Removed {{name}}", { name: plugin.fileName })), onError: (e) => toast.error(e.message) },
            )
          }
        />
      )}
    </li>
  )
}

// Installations stay below the limit of projects at once, which includes the projects they require.
const batch = 25

/** Updates all projects with a newer release, in batches. */
function UpdateAllButton({ projects, serverRef, words }: { projects: Project[]; serverRef: ServerRef; words: Words }) {
  const install = useInstallPlugins()
  async function updateAll() {
    for (let i = 0; i < projects.length; i += batch) {
      failIfAny(await install.mutateAsync({ projects: projects.slice(i, i + batch).map((p) => p.id), servers: [serverRef] }))
    }
  }
  return (
    <Button
      variant="outline"
      disabled={install.isPending}
      onClick={() =>
        toast.promise(updateAll(), { loading: t("Updating all…"), success: words.updated(projects.length), error: (e: Error) => e.message })
      }
    >
      <ArrowCircleUpIcon />
      {t("Update all ({{number}})", { number: projects.length })}
    </Button>
  )
}

function UploadButton({ serverRef }: { serverRef: ServerRef }) {
  const input = useRef<HTMLInputElement>(null)
  const change = useChangePlugins(serverRef)
  return (
    <>
      <input
        ref={input}
        type="file"
        accept=".jar"
        multiple
        hidden
        onChange={(e) => {
          for (const file of e.target.files ?? []) {
            toast.promise(change.mutateAsync({ action: "upload", file }), {
              loading: t("Uploading {{name}}…", { name: file.name }),
              success: t("Uploaded {{name}}", { name: file.name }),
              error: (err: Error) => err.message,
            })
          }
          e.target.value = ""
        }}
      />
      <Button variant="outline" disabled={change.isPending} onClick={() => input.current?.click()}>
        <UploadSimpleIcon />
        {t("Upload .jar")}
      </Button>
    </>
  )
}

function AddDialog({ server, serverRef, words, installed }: { server: Server; serverRef: ServerRef; words: Words; installed: string[] }) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button>
          <PlusIcon />
          {words.add}
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{words.addTitle}</DialogTitle>
          <DialogDescription>{words.addDescription}</DialogDescription>
        </DialogHeader>
        <PluginSearch
          type={server.type}
          version={server.version}
          autoFocus
          action={(hit) => <InstallButton hit={hit} server={server} serverRef={serverRef} installed={installed.includes(hit.id)} />}
        />
      </DialogContent>
    </Dialog>
  )
}

/** Installs the newest suitable release, or a version chosen from the menu next to it. */
function InstallButton({ hit, server, serverRef, installed }: { hit: SearchHit; server: Server; serverRef: ServerRef; installed: boolean }) {
  const install = useInstallOn(serverRef)
  if (installed)
    return (
      <Pill tone="success">
        <CheckIcon weight="bold" />
        {t("Installed")}
      </Pill>
    )
  return (
    <div className="flex">
      <Button size="sm" variant="outline" className="rounded-r-none" disabled={install.pending} onClick={() => install.run(hit)}>
        <DownloadSimpleIcon />
        {install.pending ? t("Installing…") : t("Install")}
      </Button>
      <VersionMenu project={hit.id} type={server.type} version={server.version} onPick={(v) => install.run(hit, v)}>
        <Button
          size="icon-sm"
          variant="outline"
          className="-ml-px rounded-l-none"
          disabled={install.pending}
          aria-label={t("Install another version of {{name}}", { name: hit.title })}
        >
          <CaretDownIcon />
        </Button>
      </VersionMenu>
    </div>
  )
}

/** Throws the error of a single-server installation, or returns the installed files. */
function failIfAny([result]: { error?: string; installed: { fileName: string }[] }[]): string {
  if (result.error) throw new Error(result.error)
  return result.installed.map((i) => i.fileName).join(", ")
}
