import { useQuery } from "@tanstack/react-query"
import { Link, useNavigate } from "@tanstack/react-router"
import { type FormEvent, useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { setupUserQuery, useSetup } from "./api"
import { AuthLayout } from "./auth-layout"

/** Where invited users, and users whose password was reset, set their password. */
export function SetupPage() {
  const [token] = useState(() => location.hash.slice(1))
  // The token leaves the address bar and the history once it was read.
  useEffect(() => history.replaceState(null, "", location.pathname), [])
  const { data: user, isPending, error } = useQuery({ ...setupUserQuery(token), enabled: token !== "" })

  if (!token || error) {
    return (
      <AuthLayout title="This link doesn't work" description="Setup links work once and expire after three days.">
        <p className="text-sm text-muted-foreground">
          {error?.message ?? "The link is incomplete."} Ask an administrator for a new setup link, or{" "}
          <Link to="/login" search={{}} className="font-medium text-foreground underline-offset-4 hover:underline">
            sign in
          </Link>{" "}
          if you have a password already.
        </p>
      </AuthLayout>
    )
  }
  return (
    <AuthLayout title={user ? `Welcome, ${user.username}` : "Welcome"} description="Choose the password for your account.">
      {isPending ? <Skeleton className="h-52 rounded-xl" /> : <PasswordForm token={token} username={user.username} />}
    </AuthLayout>
  )
}

function PasswordForm({ token, username }: { token: string; username: string }) {
  const [form, setForm] = useState({ password: "", repeat: "" })
  const setup = useSetup()
  const navigate = useNavigate()
  const mismatch = form.repeat !== "" && form.repeat !== form.password

  function submit(event: FormEvent) {
    event.preventDefault()
    if (!mismatch) setup.mutate({ token, password: form.password }, { onSuccess: () => navigate({ to: "/" }) })
  }

  return (
    <form onSubmit={submit}>
      <FieldGroup>
        {/* Lets password managers save the new password for the right account. */}
        <input type="text" name="username" autoComplete="username" value={username} readOnly hidden />
        <Field>
          <FieldLabel htmlFor="setup-password">Password</FieldLabel>
          <Input
            id="setup-password"
            type="password"
            autoComplete="new-password"
            autoFocus
            required
            minLength={12}
            value={form.password}
            onChange={(e) => setForm({ ...form, password: e.target.value })}
          />
          <FieldDescription>At least 12 characters.</FieldDescription>
        </Field>
        <Field data-invalid={mismatch}>
          <FieldLabel htmlFor="setup-repeat">Repeat password</FieldLabel>
          <Input
            id="setup-repeat"
            type="password"
            autoComplete="new-password"
            required
            aria-invalid={mismatch}
            value={form.repeat}
            onChange={(e) => setForm({ ...form, repeat: e.target.value })}
          />
          {mismatch && <FieldError>The passwords don't match.</FieldError>}
        </Field>
        {setup.error && <FieldError>{setup.error.message}</FieldError>}
        <Button type="submit" size="lg" className="w-full" disabled={setup.isPending || mismatch}>
          {setup.isPending ? "Saving…" : "Set password and sign in"}
        </Button>
      </FieldGroup>
    </form>
  )
}
