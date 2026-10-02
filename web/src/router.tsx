import { QueryCache, QueryClient } from "@tanstack/react-query"
import { createRootRouteWithContext, createRoute, createRouter, lazyRouteComponent, Outlet, redirect } from "@tanstack/react-router"
import { AppShell } from "@/components/app-shell"
import { home } from "@/components/navigation"
import { Toaster } from "@/components/ui/sonner"
import { accessQuery } from "@/features/access/api"
import { accessOf } from "@/features/access/use-access"
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

// Users set their password here with a setup link; the token is in the fragment.
const setupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/setup",
  component: lazyRouteComponent(() => import("@/features/auth/setup-page"), "SetupPage"),
})

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "_app",
  // The permissions are loaded first, so that the panel only offers what the user may do.
  beforeLoad: async ({ context, location }) => {
    try {
      await Promise.all([context.queryClient.ensureQueryData(meQuery), context.queryClient.ensureQueryData(accessQuery)])
    } catch {
      throw redirect({ to: "/login", search: { redirect: location.href } })
    }
  },
  component: AppShell,
})

const indexRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/",
  beforeLoad: ({ context }) => {
    throw redirect({ to: home(accessOf(context.queryClient.getQueryData(accessQuery.queryKey))) })
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

const settingsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/settings",
  component: lazyRouteComponent(() => import("@/features/settings/settings-layout"), "SettingsLayout"),
})
const generalSettingsRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: "/",
  component: lazyRouteComponent(() => import("@/features/settings/general-page"), "GeneralSettingsPage"),
})
const agentsSettingsRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: "agents",
  component: lazyRouteComponent(() => import("@/features/settings/agents-page"), "AgentsSettingsPage"),
})
const usersRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: "users",
  component: lazyRouteComponent(() => import("@/features/access/users-page"), "UsersPage"),
})
const groupsRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: "groups",
  component: lazyRouteComponent(() => import("@/features/access/groups-page"), "GroupsPage"),
})
const newGroupRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: "groups/new",
  component: lazyRouteComponent(() => import("@/features/access/group-page"), "NewGroupPage"),
})
const groupRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: "groups/$groupId",
  component: lazyRouteComponent(() => import("@/features/access/group-page"), "GroupPage"),
})
const terminalRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: "terminal",
  // target is the ID of the node whose agent runs the commands; without it, the master does.
  validateSearch: (search: Record<string, unknown>): { target?: string } => ({
    target: typeof search.target === "string" && search.target ? search.target : undefined,
  }),
  component: lazyRouteComponent(() => import("@/features/terminal/terminal-page"), "TerminalPage"),
})

export const router = createRouter({
  routeTree: rootRoute.addChildren([
    loginRoute,
    setupRoute,
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
      settingsRoute.addChildren([
        generalSettingsRoute,
        agentsSettingsRoute,
        terminalRoute,
        usersRoute,
        groupsRoute,
        newGroupRoute,
        groupRoute,
      ]),
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
