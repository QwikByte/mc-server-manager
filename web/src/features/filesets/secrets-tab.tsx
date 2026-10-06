import { KeyIcon, PlusIcon, ShuffleIcon, TrashIcon, WarningIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { Callout } from "@/components/callout"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { EmptyState } from "@/components/empty-state"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { formatDateTime } from "@/lib/format"
import { type FileSet, type SetFile, usedSecrets, useSecret } from "./api"

// An example in an empty field, which needs no translation.
const exampleName = "db-password"

const namePattern = /^[a-z0-9][a-z0-9_-]{0,63}$/

/** The secrets of a set: only their names show, and their values can be set or replaced. */
export function SecretsTab({ set, files, editable }: { set: FileSet; files: SetFile[]; editable: boolean }) {
  const secret = useSecret(set.id)
  const [editing, setEditing] = useState<{ name?: string }>()
  const used = usedSecrets(files)
  const names = [...new Set([...set.secrets.map((s) => s.name), ...used])].sort()

  const generate = (name: string) =>
    secret.mutate(
      { name, generate: true },
      { onSuccess: () => toast.success(t("Generated a new value for {{name}}", { name })), onError: (e) => toast.error(e.message) },
    )

  return (
    <div className="space-y-6">
      <Callout icon={KeyIcon}>
        {t(
          "Values can be set but never read. The master sends them only to the agents, which fill them in and hide the files that hold them from the file manager and downloads. Plugins on the servers can read them.",
        )}
      </Callout>
      {names.length === 0 ? (
        <EmptyState
          icon={KeyIcon}
          tone="warning"
          title={t("No secrets yet")}
          description={t("Use {{placeholder}} in a file, e.g. for the password of a database, and set its value here.", {
            placeholder: "{{secret:db-password}}",
          })}
        >
          {editable && (
            <Button onClick={() => setEditing({})}>
              <PlusIcon />
              {t("Add secret")}
            </Button>
          )}
        </EmptyState>
      ) : (
        <div className="surface overflow-hidden rounded-xl">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("Secret")}</TableHead>
                <TableHead>{t("Value")}</TableHead>
                <TableHead className="sr-only">{t("Actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {names.map((name) => {
                const stored = set.secrets.find((s) => s.name === name)
                return (
                  <TableRow key={name}>
                    <TableCell className="font-mono text-sm">
                      {name}
                      {!used.includes(name) && <span className="ml-2 font-sans text-xs text-muted-foreground">{t("unused")}</span>}
                    </TableCell>
                    <TableCell className="text-sm">
                      {stored ? (
                        <span className="text-muted-foreground">{t("Set on {{time}}", { time: formatDateTime(stored.updatedAt) })}</span>
                      ) : (
                        <span className="inline-flex items-center gap-1.5 text-warning">
                          <WarningIcon className="size-4" weight="fill" />
                          {t("No value yet")}
                        </span>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      {editable && (
                        <div className="flex justify-end gap-1">
                          <Button size="sm" variant="outline" onClick={() => setEditing({ name })}>
                            {stored ? t("Replace") : t("Set value")}
                          </Button>
                          <ConfirmDialog
                            trigger={
                              <Button size="icon-sm" variant="ghost" aria-label={t("Generate a value for {{name}}", { name })} title={t("Generate a random value")}>
                                <ShuffleIcon />
                              </Button>
                            }
                            title={t("Generate a value for {{name}}?", { name })}
                            description={t(
                              "The secret gets a new random value, which nobody can read. Servers get it when the set is applied, so use it only for what the servers share among themselves.",
                            )}
                            action={t("Generate")}
                            onConfirm={() => generate(name)}
                          />
                          {stored && (
                            <ConfirmDialog
                              trigger={
                                <Button
                                  size="icon-sm"
                                  variant="ghost"
                                  aria-label={t("Delete {{name}}", { name })}
                                  className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                                >
                                  <TrashIcon />
                                </Button>
                              }
                              title={t("Delete {{name}}?", { name })}
                              description={t("Servers keep the value until the set is applied. Files that use the secret can't be applied without it.")}
                              action={t("Delete secret")}
                              destructive
                              onConfirm={() => secret.mutate({ name, remove: true }, { onError: (e) => toast.error(e.message) })}
                            />
                          )}
                        </div>
                      )}
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
          {editable && (
            <div className="border-t p-3">
              <Button size="sm" variant="ghost" onClick={() => setEditing({})}>
                <PlusIcon />
                {t("Add secret")}
              </Button>
            </div>
          )}
        </div>
      )}
      {editing && <SecretDialog name={editing.name} onClose={() => setEditing(undefined)} onSave={(name, value) => secret.mutateAsync({ name, value })} />}
    </div>
  )
}

function SecretDialog({ name: fixed, onClose, onSave }: { name?: string; onClose: () => void; onSave: (name: string, value: string) => Promise<unknown> }) {
  const [name, setName] = useState(fixed ?? "")
  const [value, setValue] = useState("")
  const [error, setError] = useState<string>()
  const [pending, setPending] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (!namePattern.test(name)) return setError(t("Secret names have up to 64 lower-case letters, digits, - and _."))
    setPending(true)
    try {
      await onSave(name, value)
      toast.success(t("Saved {{name}}", { name }))
      onClose()
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setPending(false)
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="grid gap-6" autoComplete="off">
          <DialogHeader>
            <DialogTitle>{fixed ? t("Value of {{name}}", { name: fixed }) : t("Add secret")}</DialogTitle>
            <DialogDescription>{t("Servers get the new value when the set is applied.")}</DialogDescription>
          </DialogHeader>
          {!fixed && (
            <Field>
              <FieldLabel htmlFor="secret-name">{t("Name")}</FieldLabel>
              <Input id="secret-name" className="font-mono" autoFocus required placeholder={exampleName} value={name} onChange={(e) => setName(e.target.value)} />
              <FieldDescription>{t("Files use it as {{placeholder}}.", { placeholder: `{{secret:${name || "name"}}}` })}</FieldDescription>
            </Field>
          )}
          <Field>
            <FieldLabel htmlFor="secret-value">{t("Value")}</FieldLabel>
            <Input
              id="secret-value"
              type="password"
              autoFocus={Boolean(fixed)}
              required
              maxLength={1024}
              value={value}
              onChange={(e) => setValue(e.target.value)}
            />
            {error && <FieldError>{error}</FieldError>}
          </Field>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={pending || !value}>
              {t("Save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
