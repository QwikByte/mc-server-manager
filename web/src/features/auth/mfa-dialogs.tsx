import { CopyIcon, DownloadSimpleIcon } from "@phosphor-icons/react"
import type { UseMutationResult } from "@tanstack/react-query"
import { type FormEvent, type ReactElement, type ReactNode, useState } from "react"
import { toast } from "sonner"
import { ErrorCallout } from "@/components/callout"
import { CopyField } from "@/components/copy-field"
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
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { useSetUpMfa } from "./api"
import { QrCode } from "./qr-code"

type Codes = { recoveryCodes: string[] }

/** Sets up an authenticator app with a QR code; a code of the app and the password turn it on. */
export function MfaSetupDialog({
  username,
  enable,
  onEnabled,
}: {
  username: string
  enable: UseMutationResult<Codes, Error, { password: string; code: string }>
  onEnabled: (codes: string[]) => void
}) {
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({ code: "", password: "" })
  const setup = useSetUpMfa()

  // Every setup gets a new secret.
  function onOpenChange(next: boolean) {
    setOpen(next)
    setForm({ code: "", password: "" })
    enable.reset()
    if (next) setup.mutate()
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    enable.mutate(form, {
      onSuccess: ({ recoveryCodes }) => {
        onOpenChange(false)
        onEnabled(recoveryCodes)
      },
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button>Set up</Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>Set up two-factor authentication</DialogTitle>
            <DialogDescription>
              Use an authenticator app on your phone, such as Google Authenticator, Microsoft Authenticator, Aegis or your password
              manager.
            </DialogDescription>
          </DialogHeader>
          <ol className="grid gap-6">
            <Step n={1} title="Scan this QR code with the app">
              {setup.error ? (
                <ErrorCallout error={setup.error} />
              ) : setup.data ? (
                <div className="flex flex-col items-center gap-4 sm:flex-row sm:items-start">
                  <QrCode value={setup.data.uri} label="QR code for your authenticator app" className="size-40 shrink-0" />
                  <div className="w-full min-w-0 space-y-2 text-muted-foreground">
                    <p>Can't scan it? Enter this key in the app instead.</p>
                    <CopyField label="Key" value={setup.data.secret} />
                  </div>
                </div>
              ) : (
                <Skeleton className="h-40 rounded-xl" />
              )}
            </Step>
            <Step n={2} title="Confirm with a code of the app and your password">
              <FieldGroup className="sm:grid sm:grid-cols-2 sm:gap-4">
                <input type="text" name="username" autoComplete="username" value={username} readOnly hidden />
                <Field>
                  <FieldLabel htmlFor="mfa-code">Code</FieldLabel>
                  <Input
                    id="mfa-code"
                    autoComplete="one-time-code"
                    inputMode="numeric"
                    maxLength={6}
                    placeholder="000000"
                    className="font-mono tracking-[0.3em] placeholder:text-muted-foreground/40"
                    value={form.code}
                    onChange={(e) => setForm({ ...form, code: e.target.value.replace(/\D/g, "") })}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="mfa-password">Password</FieldLabel>
                  <Input
                    id="mfa-password"
                    type="password"
                    autoComplete="current-password"
                    value={form.password}
                    onChange={(e) => setForm({ ...form, password: e.target.value })}
                  />
                </Field>
              </FieldGroup>
            </Step>
          </ol>
          {enable.error && <FieldError>{enable.error.message}</FieldError>}
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">Cancel</Button>
            </DialogClose>
            <Button type="submit" disabled={!setup.data || enable.isPending || form.code.length !== 6 || !form.password}>
              {enable.isPending ? "Turning on…" : "Turn on"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function Step({ n, title, children }: { n: number; title: string; children: ReactNode }) {
  return (
    <li className="grid gap-3">
      <p className="flex items-center gap-2.5 font-medium">
        <span aria-hidden className="grid size-6 place-items-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
          {n}
        </span>
        {title}
      </p>
      {children}
    </li>
  )
}

/** Asks for the password before a change of two-factor authentication. */
export function ConfirmPasswordDialog<R>({
  trigger,
  title,
  description,
  action,
  destructive = false,
  change,
  onSuccess,
}: {
  trigger: ReactElement
  title: string
  description: string
  action: string
  destructive?: boolean
  change: UseMutationResult<R, Error, string>
  onSuccess: (result: R) => void
}) {
  const [open, setOpen] = useState(false)
  const [password, setPassword] = useState("")

  function onOpenChange(next: boolean) {
    setOpen(next)
    setPassword("")
    change.reset()
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    change.mutate(password, {
      onSuccess: (result) => {
        onOpenChange(false)
        onSuccess(result)
      },
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{description}</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="confirm-password">Password</FieldLabel>
              <Input
                id="confirm-password"
                type="password"
                autoComplete="current-password"
                autoFocus
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </Field>
            {change.error && <FieldError>{change.error.message}</FieldError>}
          </FieldGroup>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">Cancel</Button>
            </DialogClose>
            <Button type="submit" variant={destructive ? "destructive" : "default"} disabled={change.isPending || !password}>
              {action}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

/** New recovery codes, which are shown only once. It stays open until the user confirms. */
export function RecoveryCodesDialog({ codes, onClose }: { codes?: string[]; onClose: () => void }) {
  const text = codes?.join("\n") ?? ""

  async function copy() {
    await navigator.clipboard.writeText(text)
    toast.success("Copied the recovery codes")
  }

  function download() {
    const url = URL.createObjectURL(new Blob([`${text}\n`], { type: "text/plain" }))
    Object.assign(document.createElement("a"), { href: url, download: "mcsm-recovery-codes.txt" }).click()
    setTimeout(() => URL.revokeObjectURL(url))
  }

  return (
    <Dialog open={codes !== undefined} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md" showCloseButton={false} onInteractOutside={(e) => e.preventDefault()}>
        <DialogHeader>
          <DialogTitle>Save your recovery codes</DialogTitle>
          <DialogDescription>
            If you lose your phone, each code signs you in once instead of a code of the app. They are shown only now, so keep them
            somewhere safe, such as your password manager.
          </DialogDescription>
        </DialogHeader>
        <ol className="grid grid-cols-2 gap-x-6 gap-y-2 rounded-xl bg-console px-5 py-4 text-center font-mono text-sm text-console-foreground">
          {codes?.map((code) => (
            <li key={code}>{code}</li>
          ))}
        </ol>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={copy}>
            <CopyIcon /> Copy
          </Button>
          <Button variant="outline" size="sm" onClick={download}>
            <DownloadSimpleIcon /> Download
          </Button>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button>I saved them</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
