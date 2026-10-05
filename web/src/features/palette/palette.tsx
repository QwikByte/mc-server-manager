import { ArrowClockwiseIcon, GraphIcon, HardDrivesIcon, PlayIcon, StopIcon, TerminalIcon, UserIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { useState } from "react"
import { toast } from "sonner"
import { IconTile } from "@/components/icon-tile"
import { navigation } from "@/components/navigation"
import { StatusDot } from "@/components/status"
import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandShortcut,
} from "@/components/ui/command"
import type { Permission } from "@/features/access/permissions"
import { useAccess } from "@/features/access/use-access"
import { networksQuery } from "@/features/networks/api"
import { nodesQuery } from "@/features/nodes/api"
import { useOnlinePlayers } from "@/features/players/online"
import { allServersQuery, type NodeServer, serverKey, useBulkAction } from "@/features/servers/api"
import { serverLook, serverType, statusOf } from "@/features/servers/server-types"
import { msg } from "@/lib/i18n"

/** Actions on a server that the palette offers once a search starts with their name, e.g. "restart lobby". */
const actions = [
  { kind: "console", icon: TerminalIcon, label: () => t("Console"), permission: "console.view", when: () => true },
  { kind: "start", icon: PlayIcon, label: () => t("Start"), permission: "servers.start", when: (s: NodeServer) => s.state === "stopped" },
  {
    kind: "restart",
    icon: ArrowClockwiseIcon,
    label: () => t("Restart"),
    permission: "servers.restart",
    when: (s: NodeServer) => s.state !== "stopped",
  },
  { kind: "stop", icon: StopIcon, label: () => t("Stop"), permission: "servers.stop", when: (s: NodeServer) => s.state !== "stopped" },
] as const satisfies readonly { permission: Permission; [key: string]: unknown }[]

/** Searches servers, players, networks, nodes and pages, and acts on servers. */
export function Palette({ onClose }: { onClose: () => void }) {
  const access = useAccess()
  const navigate = useNavigate()
  const bulk = useBulkAction()
  const [query, setQuery] = useState("")
  const { data: servers = [] } = useQuery({ ...allServersQuery, enabled: access.canSomewhere("servers.view") })
  const { data: networks = [] } = useQuery({ ...networksQuery, enabled: access.can("networks.view") })
  const { data: nodes = [] } = useQuery({
    ...nodesQuery,
    enabled: access.canSomewhere("nodes.view") || access.canSomewhere("servers.view"),
  })
  const { players } = useOnlinePlayers(access.canSomewhere("servers.view"))
  const needle = query.trim().toLowerCase()
  const found = needle.length >= 2 ? players.filter((p) => p.name.toLowerCase().includes(needle)).slice(0, 8) : []
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  // "neustart" names "Neu starten" too.
  const named = actions.filter((a) => words.some((w) => w.length >= 3 && a.label().toLowerCase().replace(/\s/g, "").startsWith(w)))
  const pages = [
    ...navigation.flatMap((g) => g.links.filter((l) => l.visible(access))),
    { to: "/account", label: msg("Your account"), icon: UserIcon },
  ] as const

  function go(to: () => Promise<void>) {
    onClose()
    void to()
  }

  function act(kind: (typeof actions)[number]["kind"], server: NodeServer) {
    const params = { nodeId: server.nodeId, serverId: server.id }
    if (kind === "console") return go(() => navigate({ to: "/nodes/$nodeId/servers/$serverId", params }))
    onClose()
    toast.promise(
      bulk.mutateAsync({ action: kind, servers: [server] }).then(([r]) => {
        if (r?.error) throw new Error(r.error)
      }),
      {
        loading: t("{{action}}: {{name}}…", { action: actions.find((a) => a.kind === kind)!.label(), name: server.name }),
        success: t("Done: {{name}}", { name: server.name }),
        error: (e: Error) => e.message,
      },
    )
  }

  return (
    <CommandDialog
      open
      onOpenChange={(open) => !open && onClose()}
      title={t("Search")}
      description={t("Search servers, players, networks, nodes and pages")}
      className="sm:max-w-xl"
    >
      <Command loop>
        <CommandInput value={query} onValueChange={setQuery} placeholder={t("Search servers, players, networks, nodes and pages…")} />
        <CommandList className="max-h-[min(28rem,60svh)]">
          <CommandEmpty>{t("Nothing found.")}</CommandEmpty>
          {named.length > 0 && (
            <CommandGroup heading={t("Actions")}>
              {named.flatMap((a) =>
                servers
                  .filter((s) => a.when(s) && access.can(a.permission, s.nodeId, s.id))
                  .map((s) => (
                    <CommandItem
                      key={`${a.kind}/${serverKey(s)}`}
                      value={`${a.kind}/${serverKey(s)}`}
                      keywords={[a.label(), a.label().replace(/\s/g, ""), s.name, s.nodeName, ...s.tags]}
                      onSelect={() => act(a.kind, s)}
                    >
                      <a.icon weight="duotone" />
                      <span className="truncate">
                        {a.label()}: <span className="font-medium">{s.name}</span>
                      </span>
                      <CommandShortcut className="tracking-normal">{s.nodeName}</CommandShortcut>
                    </CommandItem>
                  )),
              )}
            </CommandGroup>
          )}
          {servers.length > 0 && (
            <CommandGroup heading={t("Servers")}>
              {servers.map((s) => (
                <CommandItem
                  key={serverKey(s)}
                  value={serverKey(s)}
                  keywords={[s.name, serverType(s.type).label, s.nodeName, String(s.port), ...s.tags]}
                  onSelect={() =>
                    go(() => navigate({ to: "/nodes/$nodeId/servers/$serverId", params: { nodeId: s.nodeId, serverId: s.id } }))
                  }
                >
                  <IconTile {...serverLook(s.type)} size="sm" className="size-6 rounded-md [&>svg]:size-3.5" />
                  <span className="truncate font-medium">{s.name}</span>
                  <StatusDot status={statusOf(s)} label={t(statusOf(s).label)} />
                  <CommandShortcut className="truncate tracking-normal">
                    {[...s.tags.map((tag) => `#${tag}`), s.nodeName].join(" · ")}
                  </CommandShortcut>
                </CommandItem>
              ))}
            </CommandGroup>
          )}
          {found.length > 0 && (
            <CommandGroup heading={t("Players")}>
              {found.map((p) => (
                <CommandItem
                  key={`player/${p.name}@${serverKey(p.server)}`}
                  value={`player/${p.name}@${serverKey(p.server)}`}
                  keywords={[p.name]}
                  onSelect={() => go(() => navigate({ to: "/players", search: { q: p.name } }))}
                >
                  <UserIcon weight="duotone" className="text-info" />
                  <span className="truncate font-mono font-medium">{p.name}</span>
                  <CommandShortcut className="truncate tracking-normal">{p.server.name}</CommandShortcut>
                </CommandItem>
              ))}
            </CommandGroup>
          )}
          {networks.length > 0 && (
            <CommandGroup heading={t("Networks")}>
              {networks.map((n) => (
                <CommandItem
                  key={n.id}
                  value={`network/${n.id}`}
                  keywords={[n.name, serverType(n.proxyType).label]}
                  onSelect={() => go(() => navigate({ to: "/networks/$networkId", params: { networkId: n.id } }))}
                >
                  <GraphIcon weight="duotone" className="text-violet" />
                  <span className="truncate font-medium">{n.name}</span>
                  <CommandShortcut className="tracking-normal">
                    {t("{{count}} servers", { count: n.backends.length, defaultValue_one: "{{count}} server" })}
                  </CommandShortcut>
                </CommandItem>
              ))}
            </CommandGroup>
          )}
          {nodes.length > 0 && (
            <CommandGroup heading={t("Nodes")}>
              {nodes.map((n) => (
                <CommandItem
                  key={n.id}
                  value={`node/${n.id}`}
                  keywords={[n.name, n.address ?? ""]}
                  onSelect={() => go(() => navigate({ to: "/nodes/$nodeId", params: { nodeId: n.id } }))}
                >
                  <HardDrivesIcon weight="duotone" className="text-info" />
                  <span className="truncate font-medium">{n.name}</span>
                  <StatusDot
                    status={{ tone: n.status === "online" ? "success" : n.status === "pending" ? "warning" : "destructive", label: "" }}
                  />
                </CommandItem>
              ))}
            </CommandGroup>
          )}
          <CommandGroup heading={t("Pages")}>
            {pages.map(({ to, label, icon: Icon }) => (
              <CommandItem key={to} value={`page${to}`} keywords={[t(label)]} onSelect={() => go(() => navigate({ to }))}>
                <Icon weight="duotone" />
                {t(label)}
              </CommandItem>
            ))}
          </CommandGroup>
        </CommandList>
        <p className="border-t px-3 py-2 text-xs text-muted-foreground">{t("Tip: type an action and a server, e.g. “restart lobby”.")}</p>
      </Command>
    </CommandDialog>
  )
}
