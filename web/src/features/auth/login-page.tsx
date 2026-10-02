import { getRouteApi, useNavigate } from "@tanstack/react-router"
import { type FormEvent, useState } from "react"
import { Button } from "@/components/ui/button"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { useLogin } from "./api"
import { AuthLayout } from "./auth-layout"

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
    <AuthLayout title="Welcome back" description="Sign in to manage your nodes, servers and networks.">
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
          <Button type="submit" size="lg" className="w-full" disabled={login.isPending}>
            {login.isPending ? "Signing in…" : "Sign in"}
          </Button>
        </FieldGroup>
      </form>
    </AuthLayout>
  )
}
