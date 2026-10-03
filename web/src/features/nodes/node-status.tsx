import { type Status, StatusBadge } from "@/components/status"
import { msg } from "@/lib/i18n"
import type { NodeStatus } from "./api"

const statuses: Record<NodeStatus, Status> = {
  online: { tone: "success", label: msg("Online") },
  offline: { tone: "destructive", label: msg("Offline") },
  pending: { tone: "warning", label: msg("Waiting for agent"), pulse: true },
}

export function NodeStatusBadge({ status }: { status: NodeStatus }) {
  return <StatusBadge status={statuses[status]} />
}
