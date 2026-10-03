import { PlusIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
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
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { allServersQuery } from "@/features/servers/api"
import { networksQuery, type ServerRef, useCreateNetwork } from "./api"
import { ServerSelect } from "./server-select"
import { availableServers, backendTypes, proxyTypes } from "./servers"

export function CreateNetworkDialog() {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [proxy, setProxy] = useState<ServerRef>()
  const [lobby, setLobby] = useState<ServerRef>()
  const { data: servers } = useQuery({ ...allServersQuery, enabled: open })
  const { data: networks } = useQuery(networksQuery)
  const create = useCreateNetwork()
  const navigate = useNavigate()

  function onOpenChange(next: boolean) {
    setOpen(next)
    if (!next) {
      create.reset()
      setName("")
      setProxy(undefined)
      setLobby(undefined)
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    if (!proxy || !lobby) return
    create.mutate(
      { name: name.trim(), proxy, lobby },
      {
        onSuccess: (network) => {
          toast.success(t("Created {{name}}", { name: network.name }))
          void navigate({ to: "/networks/$networkId", params: { networkId: network.id } })
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button>
          <PlusIcon />
          {t("Create network")}
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{t("Create network")}</DialogTitle>
            <DialogDescription>
              {t("Connect a Velocity proxy with the server players join first. You can add more servers afterwards.")}
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="network-name">{t("Name")}</FieldLabel>
              <Input
                id="network-name"
                placeholder={t("Main network")}
                required
                maxLength={64}
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="network-proxy">{t("Proxy")}</FieldLabel>
              <ServerSelect
                id="network-proxy"
                servers={availableServers(servers, networks, proxyTypes)}
                value={proxy}
                onChange={setProxy}
                placeholder={t("Choose a Velocity proxy")}
              />
              <FieldDescription>
                {t("Players connect to the proxy. BungeeCord is not supported, as its forwarding can be spoofed.")}
              </FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor="network-lobby">{t("Lobby")}</FieldLabel>
              <ServerSelect
                id="network-lobby"
                servers={availableServers(servers, networks, backendTypes)}
                value={lobby}
                onChange={setLobby}
                placeholder={t("Choose a Paper or Purpur server")}
              />
              <FieldDescription>
                {t("Both servers restart. The lobby then only accepts players who join through the proxy.")}
              </FieldDescription>
            </Field>
            {create.error && <FieldError>{create.error.message}</FieldError>}
          </FieldGroup>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={create.isPending || !proxy || !lobby}>
              {create.isPending ? t("Creating…") : t("Create network")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
