import { CalendarCheckIcon, PencilSimpleIcon, PlusIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { TaskCard } from "@/features/schedules/task-card"
import { actions, describePolicy, type PolicySettings, policies } from "./api"

function NewPolicy() {
  return (
    <Button asChild>
      <Link to="/policies/new">
        <PlusIcon />
        {t("New policy")}
      </Link>
    </Button>
  )
}

/** What running a policy now does, if it disrupts players. */
function confirmRun({ action, warnings }: PolicySettings) {
  const minutes = Math.max(...warnings)
  if (action === "restart")
    return warnings.length > 0
      ? t("Its running servers restart in {{minutes}} minutes, after warning the players.", { minutes })
      : t("Its running servers restart right away.")
  if (action === "stop")
    return warnings.length > 0
      ? t("Its running servers stop in {{minutes}} minutes, after warning the players.", { minutes })
      : t("Its running servers stop right away.")
}

export function PoliciesPage() {
  const manage = useAccess().can("policies.manage")
  const { data: list, isPending, error } = useQuery(policies.tasksQuery)
  return (
    <>
      <PageHeader
        icon={CalendarCheckIcon}
        tone="warning"
        title={t("Policies")}
        description={t(
          "Rules for servers or whole nodes: restart them every night, keep opening hours or run console commands at set times.",
        )}
        actions={manage && <NewPolicy />}
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
          title={t("No policies yet")}
          description={t("Create a policy to restart servers every night at 4:00, for example, with a countdown for the players.")}
        >
          {manage && <NewPolicy />}
        </EmptyState>
      ) : (
        <ul className="grid gap-4 md:grid-cols-2">
          {list.map((policy) => (
            <TaskCard
              manage={manage}
              key={policy.id}
              task={policy}
              taskApi={policies}
              icon={actions[policy.settings.action].icon}
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
                    {t("Edit")}
                  </Link>
                </Button>
              }
              confirmRun={confirmRun(policy.settings)}
            />
          ))}
        </ul>
      )}
    </>
  )
}
