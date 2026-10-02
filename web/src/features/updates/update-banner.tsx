import { ArrowsClockwiseIcon, DownloadSimpleIcon, HardDrivesIcon, SparkleIcon, WarningCircleIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { useEffect, useRef } from "react"
import { toast } from "sonner"
import { Callout } from "@/components/callout"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { CopyField } from "@/components/copy-field"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { formatDate } from "@/lib/format"
import { type Release, type UpdateStatus, updateCommand, updateQuery, updating, useUpdateAction } from "./api"

const onError = { onError: (e: Error) => toast.error(e.message) }

/**
 * Tells administrators about a new release and installs it, and about agents that are older
 * than the master. The panel reloads once the master runs another version.
 */
export function UpdateBanner() {
  const { data } = useQuery({
    ...updateQuery,
    retry: false, // the master is away while it restarts
    refetchInterval: (query) => (updating(query.state.data) ? 3_000 : 60_000),
  })
  const loaded = useRef<string>(undefined)
  useEffect(() => {
    if (!data) return
    loaded.current ??= data.version
    if (data.version !== loaded.current) window.location.reload()
  }, [data])

  if (!data || !(data.master || data.latest || data.agents.length > 0)) return null
  return (
    <div className="mb-8 space-y-4">
      {data.master ? <MasterProgress status={data} /> : data.latest && <Available status={data} release={data.latest} />}
      {data.agents.length > 0 && <OutdatedAgents status={data} />}
    </div>
  )
}

function Available({ status, release }: { status: UpdateStatus; release: Release }) {
  const install = useUpdateAction("master")
  return (
    <Callout icon={SparkleIcon} title={`MC Server Manager ${release.version} is available`}>
      <p>
        You run {status.version}. The new release was published on {formatDate(release.publishedAt)}.
        {!status.updatable && " This master wasn't installed from a package, so update it on its host:"}
      </p>
      {!status.updatable && <CopyField label="Update command" prefix="$" value={updateCommand} />}
      <div className="mt-3 flex flex-wrap gap-2">
        <ReleaseNotes release={release} />
        {status.updatable && (
          <ConfirmDialog
            trigger={
              <Button size="sm" disabled={install.isPending}>
                <DownloadSimpleIcon />
                Install now
              </Button>
            }
            title={`Install ${release.version}?`}
            description="The master installs the release and restarts, which interrupts the panel for a moment. Then it updates the agents. Minecraft servers keep running."
            action="Install now"
            onConfirm={() => install.mutate(undefined, onError)}
          />
        )}
      </div>
    </Callout>
  )
}

function MasterProgress({ status: { master, latest } }: { status: UpdateStatus }) {
  const retry = useUpdateAction("master")
  if (master?.error) {
    return (
      <Callout tone="destructive" icon={WarningCircleIcon} role="alert" title="The update of the master failed">
        <p>{master.error}</p>
        {latest && (
          <Button size="sm" variant="outline" className="mt-3" disabled={retry.isPending} onClick={() => retry.mutate(undefined, onError)}>
            Try again
          </Button>
        )}
      </Callout>
    )
  }
  return (
    <Callout icon={ArrowsClockwiseIcon} role="status" title={`Installing ${latest?.version ?? "the update"}…`}>
      The master restarts on the new release, then the panel reloads and the agents are updated. Minecraft servers keep running.
    </Callout>
  )
}

function OutdatedAgents({ status: { version, agents } }: { status: UpdateStatus }) {
  const update = useUpdateAction("agents")
  const waiting = agents.some((a) => !a.update || a.update.error)
  return (
    <Callout
      tone="warning"
      icon={HardDrivesIcon}
      title={agents.length === 1 ? "An agent is older than the master" : `${agents.length} agents are older than the master`}
    >
      <ul className="space-y-1">
        {agents.map((a) => (
          <li key={a.nodeId}>
            <Link to="/nodes/$nodeId" params={{ nodeId: a.nodeId }} className="font-medium hover:underline">
              {a.name}
            </Link>{" "}
            <span className="font-mono text-xs">{a.version}</span>
            {a.update && (a.update.error ? <span className="text-destructive"> · {a.update.error}</span> : " · updating…")}
          </li>
        ))}
      </ul>
      {waiting && (
        <Button size="sm" className="mt-3" disabled={update.isPending} onClick={() => update.mutate(undefined, onError)}>
          <DownloadSimpleIcon />
          Update to {version}
        </Button>
      )}
    </Callout>
  )
}

function ReleaseNotes({ release }: { release: Release }) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline">
          What's new
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>What's new in {release.version}</DialogTitle>
          <DialogDescription>Published on {formatDate(release.publishedAt)}.</DialogDescription>
        </DialogHeader>
        <div className="max-h-[60vh] overflow-y-auto rounded-lg bg-muted/50 p-4 text-sm whitespace-pre-wrap">
          {release.notes.trim() || "This release has no notes."}
        </div>
        <DialogFooter>
          <Button variant="outline" asChild>
            <a href={release.url} target="_blank" rel="noreferrer">
              Open on GitHub
            </a>
          </Button>
          <DialogClose asChild>
            <Button>Close</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
