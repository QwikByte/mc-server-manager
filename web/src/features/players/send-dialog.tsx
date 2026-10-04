import { PaperPlaneTiltIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldError, FieldLabel } from "@/components/ui/field"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import type { Network } from "@/features/networks/api"
import { useSendPlayer } from "./api"

/** Sends a player to another server of their network. */
export function SendDialog({ name, network, from, onClose }: { name: string; network: Network; from?: string; onClose: () => void }) {
  const targets = network.backends.map((b) => b.name).filter((b) => b !== from)
  const [server, setServer] = useState(targets[0] ?? "")
  const send = useSendPlayer(network.id)

  function submit(event: FormEvent) {
    event.preventDefault()
    send.mutate(
      { name, server },
      {
        onSuccess: () => {
          toast.success(t("Sent {{name}} to {{server}}", { name, server }))
          onClose()
        },
      },
    )
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{t("Send {{name}} to another server", { name })}</DialogTitle>
            <DialogDescription>
              {t("The proxy of {{network}} moves the player, like with /send.", { network: network.name })}
            </DialogDescription>
          </DialogHeader>
          <Field>
            <FieldLabel htmlFor="send-server">{t("Server")}</FieldLabel>
            <Select value={server} onValueChange={setServer}>
              <SelectTrigger id="send-server" className="w-full font-mono">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {targets.map((name) => (
                  <SelectItem key={name} value={name} className="font-mono">
                    {name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {send.error && <FieldError>{send.error.message}</FieldError>}
          </Field>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={!server || send.isPending}>
              <PaperPlaneTiltIcon />
              {t("Send")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
