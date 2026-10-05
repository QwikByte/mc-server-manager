import { useQuery } from "@tanstack/react-query"
import { useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { nodeQuery } from "@/features/nodes/api"
import { OperationStatus } from "@/features/operations/operation-status"
import { guard, useOperation } from "@/features/operations/use-operation"
import { type Server, serversQuery, useDuplicateServer } from "./api"
import { nextName, serverType, suggestPort, usedPorts } from "./server-types"

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
  const operation = useOperation()
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
        usedPorts(servers),
        server.port + 1,
        node?.portMin ?? undefined,
        node?.portMax ?? undefined,
      ),
  }

  const title = t("Create {{name}} as a copy", { name: values.name })

  function close(next: boolean) {
    onOpenChange(next)
    if (!next) {
      duplicate.reset()
      operation.reset()
      setName(undefined)
      setPort(undefined)
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    const open = (id: string) => navigate({ to: "/nodes/$nodeId/servers/$serverId", params: { nodeId, serverId: id } })
    operation.run((onStart) => duplicate.mutateAsync({ id: server.id, ...values, onStart }), {
      title,
      done: (copy) => ({
        message: t("Created {{copy}}, a copy of {{name}}", { copy: copy.name, name: server.name }),
        action: { label: t("Open"), onClick: () => void open(copy.id) },
      }),
      then: (copy) => {
        close(false)
        void open(copy.id)
      },
    })
  }

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="sm:max-w-md" {...guard(duplicate.isPending)}>
        {operation.live ? (
          <OperationStatus
            op={operation.live}
            title={title}
            onBackground={() => {
              operation.background(title)
              close(false)
            }}
            onBack={() => {
              operation.reset()
              duplicate.reset()
            }}
          />
        ) : (
          <form onSubmit={submit} className="grid gap-6">
            <DialogHeader>
              <DialogTitle>{t("Duplicate {{name}}", { name: server.name })}</DialogTitle>
              <DialogDescription>
                {t("The copy gets the worlds, plugins and settings of {{name}}, but not its place in a network, so it authenticates its players itself. It starts stopped.", {
                  name: server.name,
                })}
              </DialogDescription>
            </DialogHeader>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="duplicate-name">{t("Name")}</FieldLabel>
                <Input id="duplicate-name" required maxLength={32} value={values.name} onChange={(e) => setName(e.target.value)} />
              </Field>
              <Field>
                <FieldLabel htmlFor="duplicate-port">{t("Port")}</FieldLabel>
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
                <FieldDescription>{t("Every server on a node needs its own port.")}</FieldDescription>
              </Field>
              {saves && (
                <FieldDescription>
                  {t("The server saves its worlds first and pauses saving while they are copied. Players stay connected.")}
                </FieldDescription>
              )}
              {duplicate.error && <FieldError>{duplicate.error.message}</FieldError>}
            </FieldGroup>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="outline" disabled={duplicate.isPending}>
                  {t("Cancel")}
                </Button>
              </DialogClose>
              <Button type="submit" disabled={duplicate.isPending}>
                {duplicate.isPending ? t("Copying…") : t("Duplicate server")}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  )
}
