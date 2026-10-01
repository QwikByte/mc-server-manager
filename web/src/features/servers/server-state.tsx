import { StatusBadge } from "@/components/status"
import type { ServerState } from "./api"
import { serverStates } from "./server-types"

export function ServerStateBadge({ state }: { state: ServerState }) {
  return <StatusBadge status={serverStates[state]} />
}
