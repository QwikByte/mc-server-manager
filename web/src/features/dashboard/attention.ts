import { t } from "i18next"
import type { Tone } from "@/components/tone"
import type { Network } from "@/features/networks/api"
import { type Node, memoryCapacityMb } from "@/features/nodes/api"
import type { NodeServer } from "@/features/servers/api"
import type { useUsages } from "@/features/usage/api"
import { formatMegabytes } from "@/lib/format"

/** Something that needs an operator, with where to look into it. */
export interface Problem {
  key: string
  tone: Extract<Tone, "destructive" | "warning">
  title: string
  detail: string
  link:
    | { to: "/nodes/$nodeId/servers/$serverId"; params: { nodeId: string; serverId: string } }
    | { to: "/nodes/$nodeId"; params: { nodeId: string } }
    | { to: "/networks/$networkId"; params: { networkId: string } }
}

const full = 0.9

/** What needs attention across nodes, servers and networks, the most urgent first. */
export function problemsOf(
  nodes: Node[],
  servers: NodeServer[],
  networks: Network[],
  usages: ReturnType<typeof useUsages>,
): Problem[] {
  const problems: Problem[] = []
  const add = (p: Problem) => problems.push(p)
  for (const s of servers) {
    const link = { to: "/nodes/$nodeId/servers/$serverId", params: { nodeId: s.nodeId, serverId: s.id } } as const
    const exit = s.exitCode ? t("exit code {{code}}", { code: s.exitCode }) : undefined
    if (s.state === "crashing") {
      add({
        key: `crash/${s.id}`,
        tone: "destructive",
        title: t("{{name}} keeps crashing", { name: s.name }),
        detail: [s.nodeName, exit].filter(Boolean).join(" · "),
        link,
      })
    } else if (s.state === "stopped" && s.crashes > 0) {
      add({
        key: `crashed/${s.id}`,
        tone: "warning",
        title: t("{{name}} stopped after crashing", { name: s.name }),
        detail: [s.nodeName, exit].filter(Boolean).join(" · "),
        link,
      })
    }
    if (usages.server(s.nodeId, s.id)?.offlineMode) {
      add({
        key: `offline-mode/${s.id}`,
        tone: "destructive",
        title: t("{{name}} runs in offline mode", { name: s.name }),
        detail: t("Anyone who reaches it can join under any name. Turn online-mode on in server.properties."),
        link,
      })
    }
  }
  for (const n of nodes) {
    const link = { to: "/nodes/$nodeId", params: { nodeId: n.id } } as const
    if (n.status === "offline") {
      add({
        key: `offline/${n.id}`,
        tone: "destructive",
        title: t("{{name}} is offline", { name: n.name }),
        detail: t("The master can't reach its agent."),
        link,
      })
      continue
    }
    if (n.status === "pending") {
      add({
        key: `pending/${n.id}`,
        tone: "warning",
        title: t("{{name}} waits for its agent", { name: n.name }),
        detail: t("Run the enrollment command on the node."),
        link,
      })
      continue
    }
    const capacity = memoryCapacityMb(n)
    const assigned = servers.filter((s) => s.nodeId === n.id).reduce((sum, s) => sum + s.memoryMb, 0)
    if (capacity !== undefined && assigned > capacity) {
      add({
        key: `assigned/${n.id}`,
        tone: "warning",
        title: t("{{name}} has more memory assigned than it can give", { name: n.name }),
        detail: t("{{used}} of {{total}}", { used: formatMegabytes(assigned), total: formatMegabytes(capacity) }),
        link,
      })
    }
    const usage = usages.node(n.id)
    if (usage?.memoryTotalBytes && usage.memoryUsedBytes / usage.memoryTotalBytes > full) {
      add({
        key: `memory/${n.id}`,
        tone: "warning",
        title: t("{{name}} is running out of memory", { name: n.name }),
        detail: t("{{percent}} % in use", { percent: Math.round((usage.memoryUsedBytes / usage.memoryTotalBytes) * 100) }),
        link,
      })
    }
    for (const storage of n.info?.storage ?? []) {
      if (storage.totalBytes && 1 - storage.freeBytes / storage.totalBytes > full) {
        add({
          key: `storage/${n.id}/${storage.name}`,
          tone: "warning",
          title: t("Storage {{storage}} on {{name}} is almost full", { storage: storage.name, name: n.name }),
          detail: t("{{percent}} % in use", { percent: Math.round((1 - storage.freeBytes / storage.totalBytes) * 100) }),
          link,
        })
      }
    }
  }
  for (const network of networks) {
    const proxy = servers.find((s) => s.nodeId === network.proxy.nodeId && s.id === network.proxy.serverId)
    if (
      proxy?.state === "stopped" &&
      network.backends.some((b) => servers.find((s) => s.nodeId === b.nodeId && s.id === b.serverId)?.state === "running")
    ) {
      add({
        key: `proxy/${network.id}`,
        tone: "destructive",
        title: t("The proxy of {{name}} is stopped", { name: network.name }),
        detail: t("Players can't join the network, although its servers run."),
        link: { to: "/networks/$networkId", params: { networkId: network.id } },
      })
    }
  }
  return problems.sort((a, b) => Number(a.tone !== "destructive") - Number(b.tone !== "destructive"))
}
