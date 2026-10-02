import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"

export interface User {
  id: number
  username: string
}

export const meQuery = queryOptions({
  queryKey: ["me"],
  queryFn: () => api<User>("/auth/me"),
  retry: false,
  staleTime: Infinity,
})

export function useLogin() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (credentials: { username: string; password: string }) => api<User>("/auth/login", { body: credentials }),
    onSuccess: (user) => queryClient.setQueryData(meQuery.queryKey, user),
  })
}

export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => api("/auth/logout", { method: "POST" }),
    onSettled: () => queryClient.clear(),
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

/** Sets the password with a setup link, which signs the user in. */
export function useSetup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: { token: string; password: string }) => api<User>("/auth/setup", { body: input }),
    onSuccess: (user) => {
      queryClient.clear()
      queryClient.setQueryData(meQuery.queryKey, user)
    },
  })
}

/** Changes the signed-in user's password; the user's other sessions end. */
export function useChangePassword() {
  return useMutation({ mutationFn: (input: { current: string; new: string }) => api("/auth/password", { method: "PUT", body: input }) })
}
