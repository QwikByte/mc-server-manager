import { useQuery } from "@tanstack/react-query"
import { useNavigate } from "@tanstack/react-router"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { nodeQuery } from "@/features/nodes/api"
import { type Server, serversQuery, useDuplicateServer } from "./api"
import { nextName, serverType, suggestPort } from "./server-types"

/** Copies a server with all its data into a new server on the same node. */
export function DuplicateServerDialog({
  nodeId,
  server,
  open,
  onOpenChange,
}: {
  nodeId: string
  server: Server
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { data: node } = useQuery(nodeQuery(nodeId))
  const { data: servers = [] } = useQuery(serversQuery(nodeId))
  const [name, setName] = useState<string>()
  const [port, setPort] = useState<number>()
  const duplicate = useDuplicateServer(nodeId)
  const navigate = useNavigate()
  const saves = server.state !== "stopped" && !serverType(server.type).proxy
  const values = {
    name:
      name ??
      nextName(
        server.name,
        servers.map((s) => s.name),
      ),
    port:
      port ??
      suggestPort(
        servers.map((s) => s.port),
        server.port + 1,
        node?.portMin ?? undefined,
        node?.portMax ?? undefined,
      ),
  }

  function close(next: boolean) {
    onOpenChange(next)
    if (!next) {
      duplicate.reset()
      setName(undefined)
      setPort(undefined)
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    duplicate.mutate(
      { id: server.id, ...values },
      {
        onSuccess: (copy) => {
          toast.success(`Created ${copy.name}, a copy of ${server.name}`)
          close(false)
          void navigate({ to: "/nodes/$nodeId/servers/$serverId", params: { nodeId, serverId: copy.id } })
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>Duplicate {server.name}</DialogTitle>
            <DialogDescription>
              The copy gets the worlds, plugins and settings of {server.name}, but not its place in a network. It starts stopped.
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="duplicate-name">Name</FieldLabel>
              <Input id="duplicate-name" required maxLength={32} value={values.name} onChange={(e) => setName(e.target.value)} />
            </Field>
            <Field>
              <FieldLabel htmlFor="duplicate-port">Port</FieldLabel>
              <Input
                id="duplicate-port"
                type="number"
                min={1024}
                max={65535}
                required
                className="font-mono"
                value={values.port}
                onChange={(e) => setPort(e.target.valueAsNumber || 0)}
              />
              <FieldDescription>Every server on a node needs its own port.</FieldDescription>
            </Field>
            {saves && (
              <FieldDescription>
                The server saves its worlds first and pauses saving while they are copied. Players stay connected.
              </FieldDescription>
            )}
            {duplicate.error && <FieldError>{duplicate.error.message}</FieldError>}
          </FieldGroup>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">Cancel</Button>
            </DialogClose>
            <Button type="submit" disabled={duplicate.isPending}>
              {duplicate.isPending ? "Copying…" : "Duplicate server"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
