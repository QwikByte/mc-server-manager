import { ArrowClockwiseIcon, CopyIcon, DotsThreeIcon, PlayIcon, StackIcon, StopIcon, TrashIcon } from "@phosphor-icons/react"
import { useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import type { Permission } from "@/features/access/permissions"
import { useAccess } from "@/features/access/use-access"
import { SaveTemplateDialog } from "@/features/templates/save-template-dialog"
import { type Server, type ServerAction, useServerAction } from "./api"
import { DuplicateServerDialog } from "./duplicate-server-dialog"
import { serverType } from "./server-types"

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

  const { can } = useAccess()
  const may = (p: Permission) => can(p, nodeId, server.id)
  const duplicate = can("servers.create", nodeId) && may("files.read")
  const saveTemplate = can("templates.manage") && (serverType(server.type).proxy || may("properties.edit"))
  const power = running ? may("servers.restart") || may("servers.stop") : may("servers.start")
  if (!power && !duplicate && !saveTemplate && !may("servers.delete")) return null

  return (
    <div className="flex flex-wrap items-center gap-2">
      {running ? (
        <>
          {may("servers.restart") && (
            <Button size="sm" variant="outline" disabled={mutation.isPending} onClick={() => run("restart", `Restarted ${server.name}`)}>
              <ArrowClockwiseIcon />
              Restart
            </Button>
          )}
          {may("servers.stop") && (
            <Button size="sm" variant="outline" disabled={mutation.isPending} onClick={() => run("stop", `Stopped ${server.name}`)}>
              <StopIcon />
              Stop
            </Button>
          )}
        </>
      ) : (
        may("servers.start") && (
          <Button size="sm" disabled={mutation.isPending} onClick={() => run("start", `Started ${server.name}`)}>
            <PlayIcon />
            Start
          </Button>
        )
      )}
      <div className="ml-auto flex items-center gap-1">
        {(duplicate || saveTemplate) && (
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
              {duplicate && (
                <DropdownMenuItem onSelect={() => setDialog("duplicate")}>
                  <CopyIcon />
                  Duplicate…
                </DropdownMenuItem>
              )}
              {saveTemplate && (
                <DropdownMenuItem onSelect={() => setDialog("template")}>
                  <StackIcon />
                  Save as template…
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
        {may("servers.delete") && (
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
        )}
      </div>
      {dialog === "duplicate" && <DuplicateServerDialog {...dialogProps} />}
      {dialog === "template" && <SaveTemplateDialog {...dialogProps} />}
    </div>
  )
}
