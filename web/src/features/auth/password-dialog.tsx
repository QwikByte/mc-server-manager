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
import { useChangePassword } from "./api"

const empty = { current: "", next: "", repeat: "" }

/** Changes the signed-in user's password. */
export function PasswordDialog({ username }: { username: string }) {
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState(empty)
  const change = useChangePassword()
  const mismatch = form.repeat !== "" && form.repeat !== form.next

  function onOpenChange(next: boolean) {
    setOpen(next)
    setForm(empty)
    change.reset()
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    if (mismatch) return
    change.mutate(
      { current: form.current, new: form.next },
      {
        onSuccess: () => {
          toast.success(t("Changed your password"), { description: t("You were signed out everywhere else.") })
          onOpenChange(false)
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button variant="outline">{t("Change password")}</Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{t("Change your password")}</DialogTitle>
            <DialogDescription>
              {t("Other devices where you are signed in as {{name}} are signed out.", { name: username })}
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <input type="text" name="username" autoComplete="username" value={username} readOnly hidden />
            <Field>
              <FieldLabel htmlFor="password-current">{t("Current password")}</FieldLabel>
              <Input
                id="password-current"
                type="password"
                autoComplete="current-password"
                required
                value={form.current}
                onChange={(e) => setForm({ ...form, current: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="password-new">{t("New password")}</FieldLabel>
              <Input
                id="password-new"
                type="password"
                autoComplete="new-password"
                required
                minLength={12}
                value={form.next}
                onChange={(e) => setForm({ ...form, next: e.target.value })}
              />
              <FieldDescription>{t("At least 12 characters.")}</FieldDescription>
            </Field>
            <Field data-invalid={mismatch}>
              <FieldLabel htmlFor="password-repeat">{t("Repeat new password")}</FieldLabel>
              <Input
                id="password-repeat"
                type="password"
                autoComplete="new-password"
                required
                aria-invalid={mismatch}
                value={form.repeat}
                onChange={(e) => setForm({ ...form, repeat: e.target.value })}
              />
              {mismatch && <FieldError>{t("The passwords don't match.")}</FieldError>}
            </Field>
            {change.error && <FieldError>{change.error.message}</FieldError>}
          </FieldGroup>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={change.isPending || mismatch}>
              {change.isPending ? t("Saving…") : t("Change password")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
