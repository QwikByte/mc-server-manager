import { CubeIcon, PlayIcon, StopIcon, TrashIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { toast } from "sonner"
import { Lamp, type LampState } from "@/components/lamp"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { formatMegabytes } from "@/lib/format"
import { type Server, type ServerAction, type ServerState, serversQuery, useServerAction } from "./api"
import { CreateServerDialog } from "./create-server-dialog"
import { serverType } from "./server-types"

const states: Record<ServerState, { lamp: LampState; label: string }> = {
  running: { lamp: "on", label: "Running" },
  starting: { lamp: "starting", label: "Starting" },
  stopped: { lamp: "off", label: "Stopped" },
}

export function ServerList({ nodeId }: { nodeId: string }) {
  const { data: servers, isPending, error } = useQuery(serversQuery(nodeId))

  return (
    <section className="mt-10" aria-labelledby="servers-heading">
      <div className="mb-4 flex flex-wrap items-end justify-between gap-4">
        <h2 id="servers-heading" className="heading text-xl">
          Servers
        </h2>
        {servers && servers.length > 0 && <CreateServerDialog nodeId={nodeId} />}
      </div>
      {isPending ? (
        <Skeleton className="h-32" />
      ) : error ? (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      ) : servers.length === 0 ? (
        <Empty className="border border-dashed">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <CubeIcon />
            </EmptyMedia>
            <EmptyTitle>No servers on this node</EmptyTitle>
            <EmptyDescription>Create a game server or a proxy that connects servers to a network.</EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <CreateServerDialog nodeId={nodeId} />
          </EmptyContent>
        </Empty>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="hidden sm:table-cell">Software</TableHead>
              <TableHead className="hidden md:table-cell">Port</TableHead>
              <TableHead className="hidden md:table-cell">Memory</TableHead>
              <TableHead>
                <span className="sr-only">Actions</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {servers.map((server) => (
              <ServerRow key={server.id} nodeId={nodeId} server={server} />
            ))}
          </TableBody>
        </Table>
      )}
    </section>
  )
}

function ServerRow({ nodeId, server }: { nodeId: string; server: Server }) {
  const mutation = useServerAction(nodeId)
  const state = states[server.state]
  const running = server.state !== "stopped"

  function run(action: ServerAction, done: string) {
    mutation.mutate({ id: server.id, action }, { onSuccess: () => toast.success(done), onError: (e) => toast.error(e.message) })
  }

  return (
    <TableRow>
      <TableCell className="font-medium">{server.name}</TableCell>
      <TableCell>
        <span className="inline-flex items-center gap-2">
          <Lamp state={state.lamp} />
          {state.label}
        </span>
      </TableCell>
      <TableCell className="hidden sm:table-cell">
        {serverType(server.type).label}{" "}
        <span className="text-muted-foreground">{server.version === "LATEST" ? "latest" : server.version}</span>
      </TableCell>
      <TableCell className="hidden font-mono md:table-cell">{server.port}</TableCell>
      <TableCell className="hidden md:table-cell">{formatMegabytes(server.memoryMb)}</TableCell>
      <TableCell>
        <div className="flex justify-end gap-1">
          {running ? (
            <Button size="sm" variant="outline" disabled={mutation.isPending} onClick={() => run("stop", `Stopped ${server.name}`)}>
              <StopIcon />
              Stop
            </Button>
          ) : (
            <Button size="sm" variant="outline" disabled={mutation.isPending} onClick={() => run("start", `Started ${server.name}`)}>
              <PlayIcon />
              Start
            </Button>
          )}
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button size="icon-sm" variant="ghost" aria-label={`Delete ${server.name}`} disabled={mutation.isPending}>
                <TrashIcon />
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Delete {server.name}?</AlertDialogTitle>
                <AlertDialogDescription>
                  This stops the server and permanently deletes it with all worlds, plugins and settings. This can't be undone.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction variant="destructive" onClick={() => run("delete", `Deleted ${server.name}`)}>
                  Delete server
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </TableCell>
    </TableRow>
  )
}
