import { FileTextIcon, MagnifyingGlassIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useBlocker } from "@tanstack/react-router"
import { useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
import { Skeleton } from "@/components/ui/skeleton"
import { type Server, useServer, useServerAction } from "@/features/servers/api"
import { propertiesQuery, type ServerProperties, useUpdateProperties } from "./api"
import { PropertyField } from "./property-field"
import { check, definition, definitions, groups } from "./schema"

const route = getRouteApi("/_app/nodes/$nodeId/servers/$serverId/properties")
const order = Object.keys(definitions)

/** The Properties tab of a server: server.properties as a form. */
export function PropertiesPage() {
  const { nodeId, serverId } = route.useParams()
  const { server } = useServer(nodeId, serverId)
  const { data, isPending, error } = useQuery(propertiesQuery(nodeId, serverId))

  if (!server || isPending) return <Skeleton className="h-96" />
  if (error)
    return (
      <p role="alert" className="text-sm text-destructive">
        {error.message}
      </p>
    )
  if (!data.exists)
    return (
      <Empty className="border border-dashed">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <FileTextIcon />
          </EmptyMedia>
          <EmptyTitle>No server.properties yet</EmptyTitle>
          <EmptyDescription>
            The server creates it when it starts for the first time. Start the server once, then come back.
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  return <PropertiesForm nodeId={nodeId} server={server} data={data} />
}

function PropertiesForm({ nodeId, server, data }: { nodeId: string; server: Server; data: ServerProperties }) {
  const [changes, setChanges] = useState<Record<string, string>>({})
  const [search, setSearch] = useState("")
  const update = useUpdateProperties(nodeId, server.id)
  const restart = useServerAction(nodeId)
  const locked = new Map(data.locked.map((l) => [l.key, l.reason]))
  const count = Object.keys(changes).length
  const errors = Object.fromEntries(
    Object.entries(changes).flatMap(([key, value]) => (check(key, value) ? [[key, check(key, value)]] : [])),
  )
  const blocker = useBlocker({ shouldBlockFn: () => count > 0, enableBeforeUnload: () => count > 0, withResolver: true })

  function set(key: string, value: string) {
    setChanges((current) => {
      const next = { ...current, [key]: value }
      if (value === data.properties[key]) delete next[key] // back to the saved value
      return next
    })
  }

  const term = search.trim().toLowerCase()
  const keys = Object.keys(data.properties)
    .filter((key) => !term || [key, definition(key).label, definition(key).description].some((s) => s.toLowerCase().includes(term)))
    .sort((a, b) => (order.indexOf(a) + 1 || order.length + 1) - (order.indexOf(b) + 1 || order.length + 1) || a.localeCompare(b))

  function save(andRestart: boolean) {
    update.mutate(changes, {
      onSuccess: () => {
        setChanges({})
        if (!andRestart) return toast.success("Saved. The changes apply when the server restarts.")
        toast.promise(restart.mutateAsync({ id: server.id, action: "restart" }), {
          loading: `Restarting ${server.name}…`,
          success: `Saved and restarted ${server.name}`,
          error: (e: Error) => e.message,
        })
      },
      onError: (e) => toast.error(e.message),
    })
  }

  return (
    <div className="pb-24">
      <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <InputGroup className="max-w-xs">
          <InputGroupAddon>
            <MagnifyingGlassIcon />
          </InputGroupAddon>
          <InputGroupInput
            placeholder="Search settings"
            aria-label="Search settings"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </InputGroup>
        <p className="text-sm text-muted-foreground">The server reads these settings when it starts.</p>
      </div>
      {keys.length === 0 && <p className="text-sm text-muted-foreground">No setting matches your search.</p>}
      {groups.map((group) => {
        const inGroup = keys.filter((key) => definition(key).group === group)
        if (inGroup.length === 0) return null
        return (
          <section key={group} className="mb-10" aria-label={group}>
            <h2 className="heading mb-5 border-b pb-2 text-lg">{group}</h2>
            <div className="grid gap-x-10 gap-y-6 md:grid-cols-2">
              {inGroup.map((key) => (
                <PropertyField
                  key={key}
                  name={key}
                  value={changes[key] ?? data.properties[key]}
                  locked={locked.get(key)}
                  error={errors[key]}
                  changed={key in changes}
                  onChange={(value) => set(key, value)}
                />
              ))}
            </div>
          </section>
        )
      })}
      {count > 0 && (
        <div className="sticky bottom-4 z-10 flex flex-wrap items-center justify-between gap-3 border bg-card px-4 py-3 shadow-lg">
          <p className="text-sm">
            {count} unsaved {count === 1 ? "change" : "changes"}
          </p>
          <div className="flex gap-2">
            <Button variant="ghost" onClick={() => setChanges({})}>
              Discard
            </Button>
            <Button
              variant={server.state === "stopped" ? "default" : "outline"}
              disabled={update.isPending || Object.keys(errors).length > 0}
              onClick={() => save(false)}
            >
              Save
            </Button>
            {server.state !== "stopped" && (
              <Button disabled={update.isPending || Object.keys(errors).length > 0} onClick={() => save(true)}>
                Save and restart
              </Button>
            )}
          </div>
        </div>
      )}
      <ConfirmDialog
        open={blocker.status === "blocked"}
        onOpenChange={(open) => !open && blocker.reset?.()}
        title="Discard your changes?"
        description="Your changes to the settings haven't been saved."
        action="Discard changes"
        destructive
        onConfirm={() => blocker.proceed?.()}
      />
    </div>
  )
}
