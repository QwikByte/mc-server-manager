import { t } from "i18next"
import type { Tone } from "@/components/tone"
import type { Datastore } from "@/features/datastores/api"
import type { Network } from "@/features/networks/api"
import { type Node, memoryCapacityMb } from "@/features/nodes/api"
import type { Overlay } from "@/features/overlay/api"
import { assignedMemoryMb, type NodeServer } from "@/features/servers/api"
import type { useUsages } from "@/features/usage/api"
import { formatAgo, formatDate, formatMegabytes } from "@/lib/format"
import { type AutomationTask, failed } from "./tasks"

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
    | { to: "/networks/$networkId/databases"; params: { networkId: string } }
    | AutomationTask["link"]
}

const full = 0.9
/** How long before its certificate expires a node is listed. */
const expiring = 14 * 86_400_000

/**
 * What needs attention across nodes, servers, networks, the private network of the nodes, and backup jobs and schedules,
 * the most urgent first.
 */
export function problemsOf(
  nodes: Node[],
  servers: NodeServer[],
  networks: Network[],
  usages: ReturnType<typeof useUsages>,
  overlay?: Overlay,
  datastores: Datastore[] = [],
  tasks: AutomationTask[] = [],
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
    } else if (s.unhealthy) {
      add({
        key: `unhealthy/${s.id}`,
        tone: "warning",
        title: t("{{name}} is unhealthy", { name: s.name }),
        detail: [s.nodeName, t("It runs, but its health check fails, e.g. as it hangs.")].filter(Boolean).join(" · "),
        link,
      })
    }
    if (s.refusedJvmOptions?.length) {
      add({
        key: `jvm-options/${s.id}`,
        tone: "warning",
        title: t("{{name}} starts with JVM options that are no longer allowed", { name: s.name }),
        detail: t("Remove {{options}} in its settings.", { options: s.refusedJvmOptions.join(" ") }),
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
    // The master renews certificates of online nodes a month before they expire; an offline node has to be enrolled
    // again once its certificate expired.
    const left = n.certificateExpiresAt ? Date.parse(n.certificateExpiresAt) - Date.now() : Infinity
    if (left < expiring) {
      add({
        key: `certificate/${n.id}`,
        tone: left < 0 || n.status === "offline" ? "destructive" : "warning",
        title:
          left < 0
            ? t("The certificate of {{name}} expired", { name: n.name })
            : t("The certificate of {{name}} expires {{when}}", { name: n.name, when: formatAgo(n.certificateExpiresAt!) }),
        detail:
          left < 0
            ? t("Connect it again with a new join token.")
            : n.status === "offline"
              ? t("Bring it online before {{date}}, or it has to be connected again with a new join token.", {
                  date: formatDate(n.certificateExpiresAt!),
                })
              : t("Renew it on the node's page."),
        link,
      })
    }
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
        title: t("{{name}} is waiting for its agent", { name: n.name }),
        detail: t("Run the enrollment command on the node."),
        link,
      })
      continue
    }
    const capacity = memoryCapacityMb(n)
    const assigned = assignedMemoryMb(servers.filter((s) => s.nodeId === n.id))
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
        detail: t("Players can't join the network, although its servers are running."),
        link: { to: "/networks/$networkId", params: { networkId: network.id } },
      })
    }
    if (network.applyError) {
      add({
        key: `apply/${network.id}`,
        tone: "warning",
        title: t("The proxy of {{name}} may send players to the wrong address", { name: network.name }),
        detail: t("Its servers couldn't be configured. Apply the network again."),
        link: { to: "/networks/$networkId", params: { networkId: network.id } },
      })
    }
  }
  for (const ds of datastores) {
    const network = networks.find((n) => n.id === ds.networkId)
    const link = { to: "/networks/$networkId/databases", params: { networkId: ds.networkId } } as const
    const running = [network?.proxy, ...(network?.backends ?? [])].some(
      (r) => r && servers.find((s) => s.nodeId === r.nodeId && s.id === r.serverId)?.state === "running",
    )
    if (ds.state === "unhealthy") {
      add({
        key: `datastore/${ds.id}`,
        tone: "destructive",
        title: t("The datastore {{name}} is unhealthy", { name: ds.name }),
        detail: [network?.name, ds.nodeName].filter(Boolean).join(" · "),
        link,
      })
    } else if (ds.state === "stopped" && running) {
      add({
        key: `datastore/${ds.id}`,
        tone: "warning",
        title: t("The datastore {{name}} is stopped", { name: ds.name }),
        detail: t("The servers of {{network}} run without its databases.", { network: network?.name ?? "…" }),
        link,
      })
    }
  }
  for (const { task, link } of tasks.filter((a) => failed(a.task))) {
    add({
      key: `task/${link.to}/${task.id}`,
      tone: "warning",
      title: t("The last run of {{name}} failed", { name: task.name }),
      detail: [formatAgo(task.lastRun!.at), task.lastRun!.error].join(" · "),
      link,
    })
  }
  // Offline nodes are a problem of their own.
  const nameOf = (id: string) => nodes.find((n) => n.id === id)?.name ?? id
  for (const m of overlay?.members ?? []) {
    const link = { to: "/nodes/$nodeId", params: { nodeId: m.nodeId } } as const
    if (nodes.find((n) => n.id === m.nodeId)?.status === "offline") continue
    if (m.problem) {
      add({
        key: `overlay/${m.nodeId}`,
        tone: "warning",
        title: t("{{name}} isn't configured in the private network", { name: nameOf(m.nodeId) }),
        detail: m.problem,
        link,
      })
    } else if (m.unreached?.length) {
      add({
        key: `overlay-unreached/${m.nodeId}`,
        tone: "warning",
        title: t("{{name}} doesn't reach {{nodes}} over the private network", { name: nameOf(m.nodeId), nodes: m.unreached.map(nameOf).join(", ") }),
        detail: t("No handshake for 5 minutes, e.g. as UDP port {{port}} is closed between them.", { port: overlay?.port }),
        link,
      })
    }
  }
  return problems.sort((a, b) => Number(a.tone !== "destructive") - Number(b.tone !== "destructive"))
}
