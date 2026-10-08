import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { useAccess } from "@/features/access/use-access"
import { describeSchedule } from "@/features/schedules/describe"
import { actions, describePolicy, policies } from "./api"

/** The active schedules that cover a server, also through its tags or network, each linking to its page. */
export function ServerSchedules({ nodeId, serverId }: { nodeId: string; serverId: string }) {
  const { data = [] } = useQuery({ ...policies.coveringQuery(nodeId, serverId), enabled: useAccess().can("policies.view") })
  return data
    .filter((policy) => policy.enabled)
    .map((policy) => {
      const Icon = actions[policy.settings.action].icon
      return (
        <Link
          key={policy.id}
          to="/policies/$policyId"
          params={{ policyId: policy.id }}
          title={`${t("Schedule")}: ${describePolicy(policy.settings)} · ${describeSchedule(policy.schedule)}`}
          className="inline-flex items-center gap-1.5 rounded-md bg-muted px-2 py-1 text-xs font-medium whitespace-nowrap outline-none hover:bg-muted/70 focus-visible:ring-2 focus-visible:ring-ring"
        >
          <Icon className="size-3.5 text-warning" weight="duotone" />
          {policy.name}
        </Link>
      )
    })
}
