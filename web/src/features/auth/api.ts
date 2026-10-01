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
