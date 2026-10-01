import { QueryCache, QueryClient } from "@tanstack/react-query"
import { createRootRouteWithContext, createRoute, createRouter, lazyRouteComponent, Outlet, redirect } from "@tanstack/react-router"
import { AppShell } from "@/components/app-shell"
import { Toaster } from "@/components/ui/sonner"
import { meQuery } from "@/features/auth/api"
import { LoginPage } from "@/features/auth/login-page"
import { ApiError } from "@/lib/api"

export const queryClient = new QueryClient({
  // An expired session sends the user back to the sign-in page.
  queryCache: new QueryCache({
    onError: (error, query) => {
      if (error instanceof ApiError && error.status === 401 && query.queryKey[0] !== "me") {
        queryClient.clear()
        void router.navigate({ to: "/login", search: { redirect: router.state.location.href } })
      }
    },
  }),
  // Answers of the master are final; only network failures are worth retrying.
  defaultOptions: { queries: { retry: (count, error) => !(error instanceof ApiError) && count < 2 } },
})

const rootRoute = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: () => (
    <>
      <Outlet />
      <Toaster />
    </>
  ),
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  // Only same-site paths are accepted as redirect targets. The key must be set explicitly,
  // because the router merges the validated values over the raw search parameters.
  validateSearch: (search: Record<string, unknown>): { redirect?: string } => {
    const target = search.redirect
    const safe = typeof target === "string" && target.startsWith("/") && !target.startsWith("//")
    return { redirect: safe ? target : undefined }
  },
  component: LoginPage,
})

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "_app",
  beforeLoad: async ({ context, location }) => {
    try {
      await context.queryClient.ensureQueryData(meQuery)
    } catch {
      throw redirect({ to: "/login", search: { redirect: location.href } })
    }
  },
  component: AppShell,
})

const indexRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/",
  beforeLoad: () => {
    throw redirect({ to: "/nodes" })
  },
})

// Pages are loaded on demand, which keeps the sign-in page small.
const nodesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/nodes",
  component: lazyRouteComponent(() => import("@/features/nodes/nodes-page"), "NodesPage"),
})
const nodeRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/nodes/$nodeId",
  component: lazyRouteComponent(() => import("@/features/nodes/node-page"), "NodePage"),
})

export const router = createRouter({
  routeTree: rootRoute.addChildren([loginRoute, appRoute.addChildren([indexRoute, nodesRoute, nodeRoute])]),
  context: { queryClient },
  defaultPreload: "intent",
})

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router
  }
}
