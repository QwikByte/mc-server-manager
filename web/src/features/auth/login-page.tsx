import { getRouteApi, useNavigate } from "@tanstack/react-router"
import { type FormEvent, useState } from "react"
import { Lamp } from "@/components/lamp"
import { Button } from "@/components/ui/button"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { useLogin } from "./api"

const route = getRouteApi("/login")

export function LoginPage() {
  const { redirect } = route.useSearch()
  const navigate = useNavigate()
  const login = useLogin()
  const [credentials, setCredentials] = useState({ username: "", password: "" })

  function submit(event: FormEvent) {
    event.preventDefault()
    login.mutate(credentials, { onSuccess: () => navigate({ to: redirect ?? "/nodes" }) })
  }

  return (
    <main className="mx-auto flex min-h-svh max-w-sm flex-col justify-center gap-10 px-6 py-12">
      <div className="space-y-3">
        <div className="flex items-center gap-3">
          <Lamp state="on" className="size-6" />
          <h1 className="heading text-2xl">MC Server Manager</h1>
        </div>
        <p className="text-sm text-muted-foreground">Sign in to manage your nodes and servers.</p>
      </div>
      <form onSubmit={submit} noValidate>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="username">Username</FieldLabel>
            <Input
              id="username"
              autoComplete="username"
              autoFocus
              required
              value={credentials.username}
              onChange={(e) => setCredentials({ ...credentials, username: e.target.value })}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="password">Password</FieldLabel>
            <Input
              id="password"
              type="password"
              autoComplete="current-password"
              required
              value={credentials.password}
              onChange={(e) => setCredentials({ ...credentials, password: e.target.value })}
            />
          </Field>
          {login.error && <FieldError>{login.error.message}</FieldError>}
          <Button type="submit" size="lg" disabled={login.isPending}>
            {login.isPending ? "Signing in…" : "Sign in"}
          </Button>
        </FieldGroup>
      </form>
    </main>
  )
}
