import { getRouteApi, useNavigate } from "@tanstack/react-router"
import { type FormEvent, useState } from "react"
import { Logo } from "@/components/logo"
import { ThemeToggle } from "@/components/theme-toggle"
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
    <main className="relative grid min-h-svh place-items-center overflow-hidden px-4 py-16">
      <div aria-hidden className="pointer-events-none absolute inset-0 bg-blocks" />
      <ThemeToggle className="absolute top-4 right-4 w-28" />
      <div className="relative w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-5 text-center">
          <Logo className="size-14" />
          <div className="space-y-1.5">
            <h1 className="heading text-2xl">Welcome back</h1>
            <p className="text-sm text-muted-foreground">Sign in to manage your nodes, servers and networks.</p>
          </div>
        </div>
        <form
          onSubmit={submit}
          noValidate
          className="rounded-2xl bg-card/80 p-6 shadow-xl ring-1 ring-foreground/8 backdrop-blur-xl dark:shadow-black/40"
        >
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
            <Button type="submit" size="lg" className="w-full" disabled={login.isPending}>
              {login.isPending ? "Signing in…" : "Sign in"}
            </Button>
          </FieldGroup>
        </form>
      </div>
    </main>
  )
}
