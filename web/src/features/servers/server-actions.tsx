import { ArrowClockwiseIcon, PlayIcon, StopIcon, TrashIcon } from "@phosphor-icons/react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import { type Server, type ServerAction, useServerAction } from "./api"

/** Start or stop a server and delete it after confirmation. */
export function ServerActions({ nodeId, server, onDeleted }: { nodeId: string; server: Server; onDeleted?: () => void }) {
  const mutation = useServerAction(nodeId)
  const running = server.state !== "stopped"

  function run(action: ServerAction, done: string, then?: () => void) {
    mutation.mutate(
      { id: server.id, action },
      {
        onSuccess: () => {
          toast.success(done)
          then?.()
        },
        onError: (e) => toast.error(e.message),
      },
    )
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      {running ? (
        <>
          <Button size="sm" variant="outline" disabled={mutation.isPending} onClick={() => run("restart", `Restarted ${server.name}`)}>
            <ArrowClockwiseIcon />
            Restart
          </Button>
          <Button size="sm" variant="outline" disabled={mutation.isPending} onClick={() => run("stop", `Stopped ${server.name}`)}>
            <StopIcon />
            Stop
          </Button>
        </>
      ) : (
        <Button size="sm" disabled={mutation.isPending} onClick={() => run("start", `Started ${server.name}`)}>
          <PlayIcon />
          Start
        </Button>
      )}
      <ConfirmDialog
        trigger={
          <Button
            size="icon-sm"
            variant="ghost"
            aria-label={`Delete ${server.name}`}
            title="Delete server"
            disabled={mutation.isPending}
            className="ml-auto text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
          >
            <TrashIcon />
          </Button>
        }
        title={`Delete ${server.name}?`}
        description="This stops the server and permanently deletes it with all worlds, plugins and settings. This can't be undone."
        action="Delete server"
        destructive
        onConfirm={() => run("delete", `Deleted ${server.name}`, onDeleted)}
      />
    </div>
  )
}
