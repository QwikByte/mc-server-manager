import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { ApiError, api } from "@/lib/api"
import { chooseLanguage } from "@/lib/i18n"
import { forgetCommandHistories, ownCommandHistories } from "@/lib/use-command-history"

export interface User {
  id: number
  username: string
  /** The language of the panel the user chose, e.g. de; missing follows the browser. */
  language?: string
  /** Whether signing in needs a code of an authenticator app too. */
  mfa: boolean
  /** Set while the settings require two-factor authentication of the user, who hasn't set it up yet. */
  mustSetUpMfa?: boolean
}

/** Whether the master refused a request because the user has to set up two-factor authentication first. */
export const mustSetUpMfa = (error: unknown) => error instanceof ApiError && error.code === "mfa-setup-required"

export const meQuery = queryOptions({
  queryKey: ["me"],
  queryFn: () => api<User>("/auth/me"),
  retry: false,
  staleTime: Infinity,
})

/** Stores the language the signed-in user chose, "" for the browser's, and shows it. */
export function useSetLanguage() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (language: string) => api<User>("/auth/language", { method: "PUT", body: { language } }),
    onSuccess: (user) => {
      queryClient.setQueryData(meQuery.queryKey, user)
      chooseLanguage(user.language ?? "")
    },
  })
}

/** With two-factor authentication, signing in without a code only tells that one is needed. */
export type LoginResult = User | { mfaRequired: true }

export function useLogin() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (credentials: { username: string; password: string; code?: string }) =>
      api<LoginResult>("/auth/login", { body: credentials }),
    onSuccess: (result) => {
      if ("mfaRequired" in result) return
      ownCommandHistories(result.id)
      queryClient.setQueryData(meQuery.queryKey, result)
    },
  })
}

export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => api("/auth/logout", { method: "POST" }),
    onSettled: () => {
      queryClient.clear()
      forgetCommandHistories()
    },
  })
}

/** The user a setup link belongs to; it fails for invalid, expired or used links. */
export const setupUserQuery = (token: string) =>
  queryOptions({
    queryKey: ["setup", token],
    queryFn: () => api<User>("/auth/setup/check", { body: { token } }),
    retry: false,
    staleTime: Infinity,
  })

/** Sets the password with a setup link, which signs the user in unless a code is needed too. */
export function useSetup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: { token: string; password: string }) => api<LoginResult>("/auth/setup", { body: input }),
    onSuccess: (result) => {
      queryClient.clear()
      if ("mfaRequired" in result) return
      ownCommandHistories(result.id)
      queryClient.setQueryData(meQuery.queryKey, result)
    },
  })
}

/** Changes the signed-in user's password; the user's other sessions end. */
export function useChangePassword() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: { current: string; new: string }) => api("/auth/password", { method: "PUT", body: input }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: sessionsQuery.queryKey }),
  })
}

export interface MfaStatus {
  enabled: boolean
  /** How many unused recovery codes are left. */
  recoveryCodes: number
  /** Whether the settings require two-factor authentication of the user. */
  required: boolean
}

/** A new secret for an authenticator app; uri is what its QR code contains. */
export interface MfaSetup {
  secret: string
  uri: string
}

export const mfaQuery = queryOptions({ queryKey: ["mfa"], queryFn: () => api<MfaStatus>("/auth/mfa") })

export function useSetUpMfa() {
  return useMutation({ mutationFn: () => api<MfaSetup>("/auth/mfa/setup", { method: "POST" }) })
}

/**
 * The changes of two-factor authentication need the password; some return new recovery codes. Turning it on
 * or off changes the user, e.g. whether it has to be set up, and turning it on ends the other sessions.
 */
function useMfaChange<T, R>(mutationFn: (input: T) => Promise<R>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSuccess: () =>
      Promise.all(
        [mfaQuery.queryKey, meQuery.queryKey, sessionsQuery.queryKey].map((queryKey) => queryClient.invalidateQueries({ queryKey })),
      ),
  })
}

/** Turns on two-factor authentication with a code of the app set up; the user's other sessions end. */
export const useEnableMfa = () =>
  useMfaChange((input: { password: string; code: string }) => api<{ recoveryCodes: string[] }>("/auth/mfa", { body: input }))

export const useDisableMfa = () => useMfaChange((password: string) => api("/auth/mfa", { method: "DELETE", body: { password } }))

export const useNewRecoveryCodes = () =>
  useMfaChange((password: string) => api<{ recoveryCodes: string[] }>("/auth/mfa/recovery-codes", { body: { password } }))

/** A session of the signed-in user, i.e. a browser where they are signed in. */
export interface Session {
  /** Names the session; it can't sign in. */
  id: string
  /** Missing for sessions that started before the master noted it. */
  createdAt?: string
  /** Missing for such sessions that weren't used since. */
  lastUsedAt?: string
  expiresAt: string
  /** The address, browser and operating system it was used from last, as far as the master recognised them. */
  ip?: string
  browser?: string
  os?: string
  /** The session of this browser. */
  current: boolean
}

export const sessionsQuery = queryOptions({ queryKey: ["sessions"], queryFn: () => api<Session[]>("/auth/sessions") })

/** Ends one session of the user, or without an ID all but the current one. Ending sessions needs no password. */
export function useEndSessions() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id?: string) => api(id ? `/auth/sessions/${encodeURIComponent(id)}` : "/auth/sessions", { method: "DELETE" }),
    onSettled: () => queryClient.invalidateQueries({ queryKey: sessionsQuery.queryKey }),
  })
}
