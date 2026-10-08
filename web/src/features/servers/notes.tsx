import { NoteIcon, PencilSimpleIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import { type Server, useSetNotes } from "./api"

const maxNotes = 500

type NotedServer = Pick<Server, "id" | "name" | "notes">

/** The notes of a server, as plain text, with a button to change them if canEdit. */
export function ServerNotes({ nodeId, server, canEdit }: { nodeId: string; server: NotedServer; canEdit: boolean }) {
  const [editing, setEditing] = useState(false)
  if (!server.notes) return null
  return (
    <section aria-label={t("Notes")} className="mb-6 flex items-start gap-3 rounded-xl bg-muted/50 p-4 text-sm">
      <NoteIcon className="mt-px size-5 shrink-0 text-muted-foreground" />
      <p className="min-w-0 flex-1 break-words whitespace-pre-wrap">{server.notes}</p>
      {canEdit && (
        <Button
          size="icon-sm"
          variant="ghost"
          aria-label={t("Change the notes")}
          title={t("Change the notes")}
          className="-my-1.5 text-muted-foreground"
          onClick={() => setEditing(true)}
        >
          <PencilSimpleIcon />
        </Button>
      )}
      {editing && <NotesDialog nodeId={nodeId} server={server} onOpenChange={setEditing} />}
    </section>
  )
}

/** Changes the notes of a server, e.g. what it is for or whom to ask about it, which doesn't restart it. */
export function NotesDialog({ nodeId, server, onOpenChange }: { nodeId: string; server: NotedServer; onOpenChange: (open: boolean) => void }) {
  const [notes, setNotes] = useState(server.notes ?? "")
  const save = useSetNotes(nodeId, server.id)

  function submit(event: FormEvent) {
    event.preventDefault()
    save.mutate(notes, {
      onSuccess: () => {
        toast.success(t("Saved the notes of {{name}}", { name: server.name }))
        onOpenChange(false)
      },
    })
  }

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-5">
          <DialogHeader>
            <DialogTitle>{t("Notes of {{name}}", { name: server.name })}</DialogTitle>
            <DialogDescription>{t("What the server is for or whom to ask about it. The notes show on its page and the search finds them.")}</DialogDescription>
          </DialogHeader>
          <Field>
            <FieldLabel htmlFor="server-notes" className="sr-only">
              {t("Notes")}
            </FieldLabel>
            <Textarea
              id="server-notes"
              rows={5}
              maxLength={maxNotes}
              placeholder={t("e.g. Test server for the summer event, ask Alex before deleting it")}
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
            />
            <FieldDescription className="text-right tabular-nums">
              {t("{{length}} of {{max}} characters", { length: notes.length, max: maxNotes })}
            </FieldDescription>
            {save.error && <FieldError>{save.error.message}</FieldError>}
          </Field>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={save.isPending || notes.trim() === (server.notes ?? "")}>
              <NoteIcon />
              {t("Save notes")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
