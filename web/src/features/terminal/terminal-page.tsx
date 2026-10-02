import { ShieldCheckIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi } from "@tanstack/react-router"
import { Callout } from "@/components/callout"
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
        <FieldLabel htmlFor="terminal-target">Run commands on</FieldLabel>
        <Select
          value={node?.id ?? masterTarget}
          onValueChange={(next) => navigate({ search: { target: next === masterTarget ? undefined : next }, replace: true })}
        >
          <SelectTrigger id="terminal-target" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={masterTarget}>Master</SelectItem>
            {enrolled.map((n) => (
              <SelectItem key={n.id} value={n.id}>
                <StatusDot status={{ tone: n.status === "online" ? "success" : "destructive", label: n.status }} />
                {n.name}
                <span className="text-muted-foreground">agent</span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <Terminal target={node?.id ?? masterTarget} prompt={node ? `mcsm-agent@${node.name}` : "mcsm-master"} />
      <Callout className="mt-6" icon={ShieldCheckIcon} title="Not a shell">
        The commands of an agent are those of mcsm-agent on the node. They reach it through the master's mutually authenticated
        connection, like everything the panel does, and nothing runs in a shell. Storage locations and enrollment can only be
        changed in the CLI on the node itself. The master logs every command with your user name.
      </Callout>
    </>
  )
}
