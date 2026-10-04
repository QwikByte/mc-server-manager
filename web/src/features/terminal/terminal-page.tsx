import { useQuery } from "@tanstack/react-query"
import { getRouteApi } from "@tanstack/react-router"
import { t } from "i18next"
import { StatusDot } from "@/components/status"
import { Field, FieldLabel } from "@/components/ui/field"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { nodesQuery } from "@/features/nodes/api"
import { masterTarget } from "./api"
import { Terminal } from "./terminal"

const route = getRouteApi("/_app/settings/terminal")

/** The Terminal tab: the commands of the master, or of the agent of a node chosen with ?target=. */
export function TerminalPage() {
  const { target } = route.useSearch()
  const navigate = route.useNavigate()
  const { data: nodes } = useQuery(nodesQuery)
  // Without the nodes, a command meant for a node could run on the master.
  if (target && !nodes) return <Skeleton className="h-[60vh] rounded-2xl" />

  const enrolled = nodes?.filter((n) => n.enrolledAt) ?? []
  const node = enrolled.find((n) => n.id === target)
  return (
    <>
      <Field className="mb-4 sm:w-80">
        <FieldLabel htmlFor="terminal-target">{t("Run commands on")}</FieldLabel>
        <Select
          value={node?.id ?? masterTarget}
          onValueChange={(next) => navigate({ search: { target: next === masterTarget ? undefined : next }, replace: true })}
        >
          <SelectTrigger id="terminal-target" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={masterTarget}>{t("Master")}</SelectItem>
            {enrolled.map((n) => (
              <SelectItem key={n.id} value={n.id}>
                <StatusDot status={{ tone: n.status === "online" ? "success" : "destructive", label: n.status }} />
                {n.name}
                <span className="text-muted-foreground">{t("agent")}</span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <Terminal target={node?.id ?? masterTarget} prompt={node ? `noryx-agent@${node.name}` : "noryx-master"} />
    </>
  )
}
