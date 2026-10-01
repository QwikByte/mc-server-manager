import { PlusIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { type FormEvent, useState } from "react"
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
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { allServersQuery } from "@/features/servers/api"
import { type Network, networksQuery, type ServerRef } from "./api"
import { useNetworkChange } from "./network-change"
import { ServerSelect } from "./server-select"
import { availableServers, backendTypes, findServer } from "./servers"

export function AddBackendDialog({ network }: { network: Network }) {
  const [open, setOpen] = useState(false)
  const [server, setServer] = useState<ServerRef>()
  const { data: servers } = useQuery(allServersQuery)
  const { data: networks } = useQuery(networksQuery)
  const { run, isPending } = useNetworkChange(network.id)

  function onOpenChange(next: boolean) {
    setOpen(next)
    if (!next) setServer(undefined)
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    if (!server) return
    const name = findServer(servers, server)?.name ?? "the server"
    run({ action: "add", server }, { loading: `Adding ${name}…`, success: `Added ${name} to ${network.name}` })
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button disabled={isPending}>
          <PlusIcon />
          Add server
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>Add server to {network.name}</DialogTitle>
            <DialogDescription>Players switch to it with /server and its name.</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="backend-server">Server</FieldLabel>
              <ServerSelect
                id="backend-server"
                servers={availableServers(servers, networks, backendTypes)}
                value={server}
                onChange={setServer}
                placeholder="Choose a Paper or Purpur server"
              />
              <FieldDescription>
                The server restarts and then only accepts players who join through the proxy. The proxy restarts as well, which disconnects
                all players. A server on another node must be reachable from the proxy's node at its port.
              </FieldDescription>
            </Field>
          </FieldGroup>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">Cancel</Button>
            </DialogClose>
            <Button type="submit" disabled={!server}>
              Add server
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
