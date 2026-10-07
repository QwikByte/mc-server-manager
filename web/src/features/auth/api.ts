import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { chooseLanguage } from "@/lib/i18n"
import { forgetCommandHistories } from "@/lib/use-command-history"

export interface User {
  id: number
  username: string
  /** The language of the panel the user chose, e.g. de; missing follows the browser. */
  language?: string
}

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
      if (!("mfaRequired" in result)) queryClient.setQueryData(meQuery.queryKey, result)
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
      if (!("mfaRequired" in result)) queryClient.setQueryData(meQuery.queryKey, result)
    },
  })
}

/** Changes the signed-in user's password; the user's other sessions end. */
export function useChangePassword() {
  return useMutation({ mutationFn: (input: { current: string; new: string }) => api("/auth/password", { method: "PUT", body: input }) })
}

export interface MfaStatus {
  enabled: boolean
  /** How many unused recovery codes are left. */
  recoveryCodes: number
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

/** The changes of two-factor authentication need the password; some return new recovery codes. */
function useMfaChange<T, R>(mutationFn: (input: T) => Promise<R>) {
  const queryClient = useQueryClient()
  return useMutation({ mutationFn, onSuccess: () => queryClient.invalidateQueries({ queryKey: mfaQuery.queryKey }) })
}

/** Turns on two-factor authentication with a code of the app set up; the user's other sessions end. */
export const useEnableMfa = () =>
  useMfaChange((input: { password: string; code: string }) => api<{ recoveryCodes: string[] }>("/auth/mfa", { body: input }))

export const useDisableMfa = () => useMfaChange((password: string) => api("/auth/mfa", { method: "DELETE", body: { password } }))

export const useNewRecoveryCodes = () =>
  useMfaChange((password: string) => api<{ recoveryCodes: string[] }>("/auth/mfa/recovery-codes", { body: { password } }))
