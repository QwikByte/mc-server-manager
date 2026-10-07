import { DownloadSimpleIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { type OutdatedAgent, useUpdateAgent } from "./api"

/** Updates the agent of one node to the master's version, e.g. to try a release on one node first. */
export function UpdateAgentButton({ agent, version }: { agent: OutdatedAgent; version: string }) {
  const update = useUpdateAgent()
  return (
    <Button
      size="sm"
      variant="outline"
      disabled={update.isPending}
      aria-label={t("Update the agent of {{name}} to {{version}}", { name: agent.name, version })}
      onClick={() => update.mutate(agent.nodeId, { onError: (e) => toast.error(e.message) })}
    >
      <DownloadSimpleIcon />
      {t("Update")}
    </Button>
  )
}
