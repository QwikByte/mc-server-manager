import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"

/** Asks for the name of a file or folder, e.g. to create or rename one. */
export function NameDialog({
  open,
  onOpenChange,
  title,
  label,
  initial = "",
  action,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  label: string
  initial?: string
  action: string
  onSubmit: (name: string) => Promise<unknown>
}) {
  const [name, setName] = useState(initial)
  const [error, setError] = useState<string>()
  const [pending, setPending] = useState(false)

  function change(next: boolean) {
    onOpenChange(next)
    setName(initial)
    setError(undefined)
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    const trimmed = name.trim()
    if (!trimmed || trimmed === "." || trimmed === ".." || /[/\\]/.test(trimmed)) {
      setError(t("Enter a name without slashes."))
      return
    }
    setPending(true)
    try {
      await onSubmit(trimmed)
      change(false)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setPending(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={change}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
          </DialogHeader>
          <Field>
            <FieldLabel htmlFor="file-name">{label}</FieldLabel>
            <Input id="file-name" className="font-mono" autoFocus required value={name} onChange={(e) => setName(e.target.value)} />
            {error && <FieldError>{error}</FieldError>}
          </Field>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={pending}>
              {action}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
