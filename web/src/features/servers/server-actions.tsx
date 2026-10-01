import { PlayIcon, StopIcon, TrashIcon } from "@phosphor-icons/react"
import { toast } from "sonner"
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
            <AlertDialogAction variant="destructive" onClick={() => run("delete", `Deleted ${server.name}`, onDeleted)}>
              Delete server
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
