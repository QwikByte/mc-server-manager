import { type Status, StatusBadge } from "@/components/status"
import type { NodeStatus } from "./api"

const statuses: Record<NodeStatus, Status> = {
  online: { tone: "success", label: "Online" },
  offline: { tone: "destructive", label: "Offline" },
  pending: { tone: "warning", label: "Waiting for agent", pulse: true },
}

export function NodeStatusBadge({ status }: { status: NodeStatus }) {
  return <StatusBadge status={statuses[status]} />
}
