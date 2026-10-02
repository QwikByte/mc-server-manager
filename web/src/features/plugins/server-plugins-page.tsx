import {
  ArrowCircleUpIcon,
  CheckIcon,
  DownloadSimpleIcon,
  PlusIcon,
  PuzzlePieceIcon,
  TrashIcon,
  UploadSimpleIcon,
} from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi } from "@tanstack/react-router"
import { useRef } from "react"
import { toast } from "sonner"
import { Callout, ErrorCallout } from "@/components/callout"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { EmptyState } from "@/components/empty-state"
import { Section } from "@/components/section"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import type { ServerRef } from "@/features/networks/api"
import { type Server, useServer } from "@/features/servers/api"
import { serverType } from "@/features/servers/server-types"
import { formatBytes } from "@/lib/format"
import { type InstalledPlugin, pluginsQuery, type SearchHit, useChangePlugins, useInstallPlugins } from "./api"
import { PluginIcon } from "./plugin-icon"
import { PluginSearch } from "./plugin-search"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId/plugins")

/** The Plugins tab of a server, Mods for modded servers. */
export function ServerPluginsPage() {
  const { nodeId, serverId } = route.useParams()
  const manage = useAccess().can("plugins.manage", nodeId, serverId)
  const { server } = useServer(nodeId, serverId)
  const ref = { nodeId, serverId }
  const { data, isPending, error } = useQuery(pluginsQuery(ref))

  if (!server || isPending) return <Skeleton className="h-64 rounded-xl" />
  if (error) return <ErrorCallout error={error} />
  const kind = serverType(server.type).addons?.kind ?? "plugins"
  const installed = data.plugins.flatMap((p) => (p.project ? [p.project.id] : []))

  return (
    <Section
      title={`${data.plugins.length} ${data.plugins.length === 1 ? kind.slice(0, -1) : kind}`}
      description={`Files in the ${data.folder} folder. Restart the server to load changes.`}
      className="mt-0"
      actions={
        manage && (
          <div className="flex flex-wrap gap-2">
            <UploadButton serverRef={ref} />
            <AddDialog server={server} serverRef={ref} kind={kind} installed={installed} />
          </div>
        )
      }
    >
      {data.catalogueError && (
        <Callout tone="warning" className="mb-4">
          Modrinth couldn't identify the files: {data.catalogueError}
        </Callout>
      )}
      {data.plugins.length === 0 ? (
        <EmptyState
          icon={PuzzlePieceIcon}
          tone="warning"
          title={`No ${kind} yet`}
          description={manage ? `Add ${kind} from Modrinth, or upload your own .jar files.` : `The server has no ${kind}.`}
        >
          {manage && <AddDialog server={server} serverRef={ref} kind={kind} installed={installed} />}
        </EmptyState>
      ) : (
        <ul className="surface divide-y rounded-xl">
          {data.plugins.map((plugin) => (
            <PluginRow key={plugin.fileName} plugin={plugin} serverRef={ref} manage={manage} />
          ))}
        </ul>
      )}
    </Section>
  )
}

function PluginRow({ plugin, serverRef, manage }: { plugin: InstalledPlugin; serverRef: ServerRef; manage: boolean }) {
  const change = useChangePlugins(serverRef)
  const install = useInstallPlugins()
  const { project, update } = plugin

  function updateIt() {
    if (!project) return
    toast.promise(install.mutateAsync({ projects: [project.id], servers: [serverRef] }).then(failIfAny), {
      loading: `Updating ${project.title}…`,
      success: `Updated ${project.title} to ${update}. Restart the server to load it.`,
      error: (e: Error) => e.message,
    })
  }

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
          {plugin.version && <span className="truncate font-mono text-xs font-normal text-muted-foreground">{plugin.version}</span>}
        </p>
        <p className="truncate text-xs text-muted-foreground">
          {project ? plugin.fileName : "Not from Modrinth"} · {formatBytes(plugin.size)}
        </p>
      </div>
      {update && !manage && <Pill tone="info">Update to {update}</Pill>}
      {update && manage && (
        <Button size="sm" variant="outline" disabled={install.isPending} onClick={updateIt}>
          <ArrowCircleUpIcon />
          <span className="max-sm:sr-only">Update to {update}</span>
        </Button>
      )}
      {manage && (
        <ConfirmDialog
          trigger={
            <Button
              size="icon-sm"
              variant="ghost"
              aria-label={`Remove ${plugin.fileName}`}
              title="Remove"
              disabled={change.isPending}
              className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
            >
              <TrashIcon />
            </Button>
          }
          title={`Remove ${project?.title ?? plugin.fileName}?`}
          description={`This deletes ${plugin.fileName}. Its configuration in the server's folder is kept.`}
          action="Remove"
          destructive
          onConfirm={() =>
            change.mutate(
              { action: "remove", fileName: plugin.fileName },
              { onSuccess: () => toast.success(`Removed ${plugin.fileName}`), onError: (e) => toast.error(e.message) },
            )
          }
        />
      )}
    </li>
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
              loading: `Uploading ${file.name}…`,
              success: `Uploaded ${file.name}`,
              error: (err: Error) => err.message,
            })
          }
          e.target.value = ""
        }}
      />
      <Button variant="outline" disabled={change.isPending} onClick={() => input.current?.click()}>
        <UploadSimpleIcon />
        Upload .jar
      </Button>
    </>
  )
}

function AddDialog({ server, serverRef, kind, installed }: { server: Server; serverRef: ServerRef; kind: string; installed: string[] }) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button>
          <PlusIcon />
          Add {kind}
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Add {kind} from Modrinth</DialogTitle>
          <DialogDescription>
            Only {kind} for {serverType(server.type).label} are shown. What they require is installed too.
          </DialogDescription>
        </DialogHeader>
        <PluginSearch
          type={server.type}
          version={server.version}
          autoFocus
          action={(hit) => <InstallButton hit={hit} serverRef={serverRef} installed={installed.includes(hit.id)} />}
        />
      </DialogContent>
    </Dialog>
  )
}

function InstallButton({ hit, serverRef, installed }: { hit: SearchHit; serverRef: ServerRef; installed: boolean }) {
  const install = useInstallPlugins()
  if (installed)
    return (
      <Pill tone="success">
        <CheckIcon weight="bold" />
        Installed
      </Pill>
    )
  return (
    <Button
      size="sm"
      variant="outline"
      disabled={install.isPending}
      onClick={() =>
        toast.promise(install.mutateAsync({ projects: [hit.id], servers: [serverRef] }).then(failIfAny), {
          loading: `Installing ${hit.title}…`,
          success: (files) => `Installed ${files}. Restart the server to load it.`,
          error: (e: Error) => e.message,
        })
      }
    >
      <DownloadSimpleIcon />
      {install.isPending ? "Installing…" : "Install"}
    </Button>
  )
}

/** Throws the error of a single-server installation, or returns the installed files. */
function failIfAny([result]: { error?: string; installed: { fileName: string }[] }[]): string {
  if (result.error) throw new Error(result.error)
  return result.installed.map((i) => i.fileName).join(", ")
}
