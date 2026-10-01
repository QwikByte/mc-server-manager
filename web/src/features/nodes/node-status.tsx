import { Lamp, type LampState } from "@/components/lamp"
import type { NodeStatus } from "./api"

const statuses: Record<NodeStatus, { lamp: LampState; label: string }> = {
  online: { lamp: "on", label: "Online" },
  offline: { lamp: "off", label: "Offline" },
  pending: { lamp: "unset", label: "Waiting for agent" },
}

export function NodeStatusLabel({ status }: { status: NodeStatus }) {
  const { lamp, label } = statuses[status]
  return (
    <span className="inline-flex items-center gap-2 whitespace-nowrap">
      <Lamp state={lamp} />
      {label}
    </span>
  )
}
