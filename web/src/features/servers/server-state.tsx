import { Lamp } from "@/components/lamp"
import type { ServerState } from "./api"
import { serverStates } from "./server-types"

export function ServerStateLabel({ state }: { state: ServerState }) {
  const { lamp, label } = serverStates[state]
  return (
    <span className="inline-flex items-center gap-2 whitespace-nowrap">
      <Lamp state={lamp} />
      {label}
    </span>
  )
}
