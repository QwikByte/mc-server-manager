import { ChatCircleTextIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { Segmented } from "@/components/segmented"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import type { ServerRef } from "@/features/networks/api"
import { findServer } from "@/features/networks/servers"
import { allServersQuery } from "@/features/servers/api"
import { playersLabel } from "./actions"
import { type Message, type MessageKind, useMessagePlayers } from "./api"

const maxMessage = 256

/**
 * Asks for a message that players see in the chat, as a title in the middle of the screen with an optional subtitle, or
 * above the hotbar, and sends it; chat tells how the chat shows it.
 */
export function MessageDialog({
  title,
  chat,
  onSend,
  onClose,
}: {
  title: string
  chat: string
  /** Sends the message, and tells what didn't go as planned. */
  onSend: (message: Message) => Promise<string | undefined>
  onClose: () => void
}) {
  const [message, setMessage] = useState<Message>({ kind: "chat", message: "", subtitle: "" })
  const [error, setError] = useState<string>()
  const [pending, setPending] = useState(false)
  const set = (change: Partial<Message>) => setMessage({ ...message, ...change })
  const descriptions: Record<MessageKind, string> = {
    chat: chat,
    title: t("In large letters in the middle of the screen, with a smaller subtitle below if you like."),
    actionbar: t("Above the hotbar, for a few seconds."),
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    setPending(true)
    onSend({ ...message, message: message.message.trim(), subtitle: message.kind === "title" ? message.subtitle?.trim() || undefined : undefined }).then(
      (warning) => {
        if (warning) toast.warning(t("Not everyone got the message"), { description: warning })
        else toast.success(t("Message sent"))
        onClose()
      },
      (e: Error) => {
        setError(e.message)
        setPending(false)
      },
    )
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{descriptions[message.kind]}</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Segmented<MessageKind>
              label={t("Where it shows")}
              value={message.kind}
              onChange={(kind) => set({ kind })}
              className="max-w-full self-start overflow-x-auto"
              options={[
                { value: "chat", label: t("Chat") },
                { value: "title", label: t("Title") },
                { value: "actionbar", label: t("Action bar") },
              ]}
            />
            <Field>
              <FieldLabel htmlFor="message-text">{t("Message")}</FieldLabel>
              <Input
                id="message-text"
                autoFocus
                required
                maxLength={maxMessage}
                placeholder={t("The network restarts in 5 minutes")}
                value={message.message}
                onChange={(e) => set({ message: e.target.value })}
              />
            </Field>
            {message.kind === "title" && (
              <Field>
                <FieldLabel htmlFor="message-subtitle">{t("Subtitle")}</FieldLabel>
                <Input
                  id="message-subtitle"
                  maxLength={maxMessage}
                  placeholder={t("Optional")}
                  value={message.subtitle}
                  onChange={(e) => set({ subtitle: e.target.value })}
                />
              </Field>
            )}
            {error && <FieldError>{error}</FieldError>}
          </FieldGroup>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={!message.message.trim() || pending}>
              <ChatCircleTextIcon />
              {t("Send")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

/** Sends a message to players on the servers they're on: privately in the chat, as a title or above the hotbar. */
export function PlayerMessageDialog({ names, servers, onClose }: { names: string[]; servers: ServerRef[]; onClose: () => void }) {
  const send = useMessagePlayers()
  const { data: all } = useQuery(allServersQuery)
  const nameOf = (ref: ServerRef) => findServer(all, ref)?.name ?? ref.serverId
  return (
    <MessageDialog
      title={t("Message to {{name}}", { name: playersLabel(names) })}
      chat={t("A private message of the server in the chat, like with msg.")}
      onClose={onClose}
      onSend={(message) =>
        send.mutateAsync({ ...message, names, servers }).then((results) => {
          const failed = results.filter((r) => r.error)
          return failed.length > 0 ? failed.map((r) => `${nameOf(r)}: ${r.error}`).join(" · ") : undefined
        })
      }
    />
  )
}
