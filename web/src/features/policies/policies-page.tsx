import { CalendarCheckIcon, PencilSimpleIcon, PlusIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { TaskCard } from "@/features/schedules/task-card"
import { actions, describePolicy, policies, warns } from "./api"

const newPolicy = (
  <Button asChild>
    <Link to="/policies/new">
      <PlusIcon />
      New policy
    </Link>
  </Button>
)

export function PoliciesPage() {
  const manage = useAccess().can("policies.manage")
  const { data: list, isPending, error } = useQuery(policies.tasksQuery)
  return (
    <>
      <PageHeader
        icon={CalendarCheckIcon}
        tone="warning"
        title="Policies"
        description="Rules for servers or whole nodes: restart them every night, keep opening hours or run console commands at set times."
        actions={manage && newPolicy}
      />
      {isPending ? (
        <div className="grid gap-4 md:grid-cols-2">
          {[0, 1].map((i) => (
            <Skeleton key={i} className="h-60 rounded-xl" />
          ))}
        </div>
      ) : error ? (
        <ErrorCallout error={error} />
      ) : list.length === 0 ? (
        <EmptyState
          icon={CalendarCheckIcon}
          tone="warning"
          title="No policies yet"
          description="Create a policy to restart servers every night at 4:00, for example, with a countdown for the players."
        >
          {manage && newPolicy}
        </EmptyState>
      ) : (
        <ul className="grid gap-4 md:grid-cols-2">
          {list.map((policy) => {
            const { action, warnings } = policy.settings
            return (
              <TaskCard
                manage={manage}
                key={policy.id}
                task={policy}
                taskApi={policies}
                icon={actions[action].icon}
                tone="warning"
                title={
                  <Link to="/policies/$policyId" params={{ policyId: policy.id }} className="block truncate font-semibold hover:underline">
                    {policy.name}
                  </Link>
                }
                summary={describePolicy(policy.settings)}
                edit={
                  <Button asChild size="sm" variant="outline">
                    <Link to="/policies/$policyId" params={{ policyId: policy.id }}>
                      <PencilSimpleIcon />
                      Edit
                    </Link>
                  </Button>
                }
                confirmRun={
                  warns(action)
                    ? `Its running servers ${action === "restart" ? "restart" : "stop"} ${
                        warnings.length > 0 ? `in ${Math.max(...warnings)} minutes, after warning the players` : "right away"
                      }.`
                    : undefined
                }
              />
            )
          })}
        </ul>
      )}
    </>
  )
}
