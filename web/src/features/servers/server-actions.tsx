import { ArrowClockwiseIcon, CopyIcon, DotsThreeIcon, PlayIcon, StackIcon, StopIcon, TrashIcon } from "@phosphor-icons/react"
import { useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { SaveTemplateDialog } from "@/features/templates/save-template-dialog"
import { type Server, type ServerAction, useServerAction } from "./api"
import { DuplicateServerDialog } from "./duplicate-server-dialog"

/** Start or stop a server, copy it, save it as a template and delete it after confirmation. */
export function ServerActions({ nodeId, server, onDeleted }: { nodeId: string; server: Server; onDeleted?: () => void }) {
  const mutation = useServerAction(nodeId)
  const [dialog, setDialog] = useState<"duplicate" | "template">()
  const running = server.state !== "stopped"
  const dialogProps = { nodeId, server, open: true, onOpenChange: (open: boolean) => !open && setDialog(undefined) }

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
      <div className="ml-auto flex items-center gap-1">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              size="icon-sm"
              variant="ghost"
              aria-label={`More actions for ${server.name}`}
              title="More actions"
              className="text-muted-foreground"
            >
              <DotsThreeIcon weight="bold" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-52">
            <DropdownMenuItem onSelect={() => setDialog("duplicate")}>
              <CopyIcon />
              Duplicate…
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setDialog("template")}>
              <StackIcon />
              Save as template…
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <ConfirmDialog
          trigger={
            <Button
              size="icon-sm"
              variant="ghost"
              aria-label={`Delete ${server.name}`}
              title="Delete server"
              disabled={mutation.isPending}
              className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
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
      {dialog === "duplicate" && <DuplicateServerDialog {...dialogProps} />}
      {dialog === "template" && <SaveTemplateDialog {...dialogProps} />}
    </div>
  )
}
