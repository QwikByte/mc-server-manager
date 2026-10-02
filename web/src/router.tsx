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
const serverRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/nodes/$nodeId/servers/$serverId",
  component: lazyRouteComponent(() => import("@/features/servers/server-page"), "ServerPage"),
})
const serverConsoleRoute = createRoute({
  getParentRoute: () => serverRoute,
  path: "/",
  component: lazyRouteComponent(() => import("@/features/servers/server-page"), "ServerConsole"),
})
const serverFilesRoute = createRoute({
  getParentRoute: () => serverRoute,
  path: "files",
  // path is the folder shown, edit the file open in the editor.
  validateSearch: (search: Record<string, unknown>): { path?: string; edit?: string } => ({
    path: typeof search.path === "string" && search.path ? search.path : undefined,
    edit: typeof search.edit === "string" && search.edit ? search.edit : undefined,
  }),
  component: lazyRouteComponent(() => import("@/features/files/files-page"), "FilesPage"),
})
const serverPropertiesRoute = createRoute({
  getParentRoute: () => serverRoute,
  path: "properties",
  component: lazyRouteComponent(() => import("@/features/properties/properties-page"), "PropertiesPage"),
})
const serverPluginsRoute = createRoute({
  getParentRoute: () => serverRoute,
  path: "plugins",
  component: lazyRouteComponent(() => import("@/features/plugins/server-plugins-page"), "ServerPluginsPage"),
})
const serverBackupsRoute = createRoute({
  getParentRoute: () => serverRoute,
  path: "backups",
  component: lazyRouteComponent(() => import("@/features/backups/server-backups-page"), "ServerBackupsPage"),
})
const serverSettingsRoute = createRoute({
  getParentRoute: () => serverRoute,
  path: "settings",
  component: lazyRouteComponent(() => import("@/features/servers/settings-page"), "SettingsPage"),
})

const networksRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/networks",
  component: lazyRouteComponent(() => import("@/features/networks/networks-page"), "NetworksPage"),
})
const networkRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/networks/$networkId",
  component: lazyRouteComponent(() => import("@/features/networks/network-page"), "NetworkPage"),
})

const templatesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/templates",
  component: lazyRouteComponent(() => import("@/features/templates/templates-page"), "TemplatesPage"),
})
const newTemplateRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/templates/new",
  component: lazyRouteComponent(() => import("@/features/templates/template-page"), "NewTemplatePage"),
})
const templateRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/templates/$templateId",
  component: lazyRouteComponent(() => import("@/features/templates/template-page"), "TemplatePage"),
})

const backupJobsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/backups",
  component: lazyRouteComponent(() => import("@/features/backups/backup-jobs-page"), "BackupJobsPage"),
})
const newBackupJobRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/backups/new",
  component: lazyRouteComponent(() => import("@/features/backups/backup-job-page"), "NewBackupJobPage"),
})
const backupJobRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/backups/$jobId",
  component: lazyRouteComponent(() => import("@/features/backups/backup-job-page"), "BackupJobPage"),
})

const policiesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/policies",
  component: lazyRouteComponent(() => import("@/features/policies/policies-page"), "PoliciesPage"),
})
const newPolicyRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/policies/new",
  component: lazyRouteComponent(() => import("@/features/policies/policy-page"), "NewPolicyPage"),
})
const policyRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/policies/$policyId",
  component: lazyRouteComponent(() => import("@/features/policies/policy-page"), "PolicyPage"),
})

const pluginsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/plugins",
  component: lazyRouteComponent(() => import("@/features/plugins/plugins-page"), "PluginsPage"),
})

export const router = createRouter({
  routeTree: rootRoute.addChildren([
    loginRoute,
    appRoute.addChildren([
      indexRoute,
      nodesRoute,
      nodeRoute,
      serverRoute.addChildren([
        serverConsoleRoute,
        serverFilesRoute,
        serverPropertiesRoute,
        serverPluginsRoute,
        serverBackupsRoute,
        serverSettingsRoute,
      ]),
      networksRoute,
      networkRoute,
      templatesRoute,
      newTemplateRoute,
      templateRoute,
      backupJobsRoute,
      newBackupJobRoute,
      backupJobRoute,
      policiesRoute,
      newPolicyRoute,
      policyRoute,
      pluginsRoute,
    ]),
  ]),
  context: { queryClient },
  defaultPreload: "intent",
})

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router
  }
}
