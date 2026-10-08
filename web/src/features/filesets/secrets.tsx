import { CheckCircleIcon, KeyIcon, PencilSimpleIcon, PlusIcon, ShuffleIcon, TrashIcon, WarningIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { formatDate } from "@/lib/format"
import { type FileSet, type SetFile, usedSecrets, useSecret } from "./api"
import { PanelSection } from "./panel-section"

// An example in an empty field, which needs no translation.
const exampleName = "db-password"

const namePattern = /^[a-z0-9][a-z0-9_-]{0,63}$/

/** The secrets of a set and those its files use: only their names show, and their values can be set. */
export function SecretsSection({ set, files, editable }: { set: FileSet; files: SetFile[]; editable: boolean }) {
  const secret = useSecret(set.id)
  const [editing, setEditing] = useState<{ name?: string }>()
  const used = usedSecrets(files)
  const names = [...new Set([...set.secrets.map((s) => s.name), ...used])].sort()

  return (
    <PanelSection
      icon={KeyIcon}
      title={t("Secrets")}
      action={
        editable && (
          <Button size="icon-sm" variant="ghost" aria-label={t("Add secret")} title={t("Add secret")} onClick={() => setEditing({})}>
            <PlusIcon />
          </Button>
        )
      }
    >
      {names.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {t("For passwords: use {{placeholder}} in a file, e.g. with Insert, and set its value here.", { placeholder: "{{secret:<name>}}" })}
        </p>
      ) : (
        <ul className="grid gap-1">
          {names.map((name) => {
            const stored = set.secrets.find((s) => s.name === name)
            return (
              <li key={name} className="flex items-center gap-2 rounded-lg py-1 pr-1 pl-2 ring-1 ring-border">
                {stored ? (
                  <CheckCircleIcon className="size-4 shrink-0 text-success" weight="fill" aria-label={t("Has a value")} />
                ) : (
                  <WarningIcon className="size-4 shrink-0 text-warning" weight="fill" aria-label={t("No value yet")} />
                )}
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-mono text-xs">{name}</span>
                  <span className="block text-xs text-muted-foreground">
                    {[stored ? t("Set on {{time}}", { time: formatDate(stored.updatedAt) }) : t("No value yet"), !used.includes(name) && t("unused")]
                      .filter(Boolean)
                      .join(" · ")}
                  </span>
                </span>
                {editable && (
                  <>
                    <Button size="icon-sm" variant="ghost" aria-label={t("Set the value of {{name}}", { name })} title={stored ? t("Replace") : t("Set value")} onClick={() => setEditing({ name })}>
                      <PencilSimpleIcon />
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
                      onConfirm={() =>
                        secret.mutate(
                          { name, generate: true },
                          { onSuccess: () => toast.success(t("Generated a new value for {{name}}", { name })), onError: (e) => toast.error(e.message) },
                        )
                      }
                    />
                    {stored && (
                      <ConfirmDialog
                        trigger={
                          <Button size="icon-sm" variant="ghost" aria-label={t("Delete {{name}}", { name })} className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive">
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
                  </>
                )}
              </li>
            )
          })}
        </ul>
      )}
      <p className="text-xs text-muted-foreground">
        {t("Values are saved right away and can't be read again. Only the agents fill them in, and the file manager hides the files that hold them.")}
      </p>
      {editing && <SecretDialog setId={set.id} name={editing.name} onClose={() => setEditing(undefined)} />}
    </PanelSection>
  )
}

/** Sets the value of a secret, which is named first if it is new. */
export function SecretDialog({ setId, name: fixed, onClose, onSaved }: { setId: string; name?: string; onClose: () => void; onSaved?: (name: string) => void }) {
  const secret = useSecret(setId)
  const [name, setName] = useState(fixed ?? "")
  const [value, setValue] = useState("")
  const [error, setError] = useState<string>()

  function submit(event: FormEvent) {
    event.preventDefault()
    if (!namePattern.test(name)) return setError(t("Secret names have up to 64 lower-case letters, digits, - and _."))
    secret.mutate(
      { name, value },
      {
        onSuccess: () => {
          toast.success(t("Saved {{name}}", { name }))
          onSaved?.(name)
          onClose()
        },
        onError: (e) => setError(e.message),
      },
    )
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
            <Button type="submit" disabled={secret.isPending || !value}>
              {t("Save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
