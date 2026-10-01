import { CubeIcon, SignInIcon, TrashIcon } from "@phosphor-icons/react"
import { Link } from "@tanstack/react-router"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { IconTile } from "@/components/icon-tile"
import { Section } from "@/components/section"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import type { NodeServer } from "@/features/servers/api"
import { AddBackendDialog } from "./add-backend-dialog"
import type { Backend, Network } from "./api"
import { useNetworkChange } from "./network-change"
import { ServerLabel } from "./server-label"
import { findServer } from "./servers"

const disconnects = "The proxy restarts to apply this, which disconnects all players of the network."

/** The game servers behind the proxy; players join the first one. */
export function BackendList({ network, servers }: { network: Network; servers?: NodeServer[] }) {
  return (
    <Section
      title="Servers"
      description="Players switch between them with /server and the name."
      actions={<AddBackendDialog network={network} />}
    >
      <div className="surface overflow-hidden rounded-xl">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Server</TableHead>
              <TableHead className="hidden sm:table-cell">Switch with</TableHead>
              <TableHead className="hidden md:table-cell">Port</TableHead>
              <TableHead>
                <span className="sr-only">Actions</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {network.backends.map((backend, i) => (
              <BackendRow
                key={backend.serverId}
                network={network}
                backend={backend}
                server={findServer(servers, backend)}
                first={i === 0}
              />
            ))}
          </TableBody>
        </Table>
      </div>
    </Section>
  )
}

function BackendRow({
  network,
  backend,
  server,
  first,
}: {
  network: Network
  backend: Backend
  server: NodeServer | null | undefined
  first: boolean
}) {
  const { run, isPending } = useNetworkChange(network.id)
  const name = server?.name ?? backend.name
  const last = network.backends.length === 1

  return (
    <TableRow>
      <TableCell>
        <span className="inline-flex items-center gap-3">
          <IconTile icon={CubeIcon} size="sm" />
          <Link
            to="/nodes/$nodeId/servers/$serverId"
            params={{ nodeId: backend.nodeId, serverId: backend.serverId }}
            className="hover:underline"
          >
            <ServerLabel server={server} />
          </Link>
          {first && <Pill tone="info">Players join here</Pill>}
        </span>
      </TableCell>
      <TableCell className="hidden font-mono sm:table-cell">/server {backend.name}</TableCell>
      <TableCell className="hidden font-mono md:table-cell">{server?.port ?? "–"}</TableCell>
      <TableCell>
        <div className="flex justify-end gap-1">
          {!first && (
            <ConfirmDialog
              trigger={
                <Button size="sm" variant="outline" disabled={isPending}>
                  <SignInIcon />
                  <span className="max-sm:sr-only">Join here</span>
                </Button>
              }
              title={`Let players join ${name}?`}
              description={`Players who connect to ${network.name} start on ${name}. ${disconnects}`}
              action="Let players join here"
              onConfirm={() =>
                run(
                  { action: "default", serverId: backend.serverId },
                  { loading: "Updating the proxy…", success: `Players now join ${name}` },
                )
              }
            />
          )}
          <ConfirmDialog
            trigger={
              <Button
                size="icon-sm"
                variant="ghost"
                className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                aria-label={`Remove ${name} from the network`}
                title={last ? "A network needs at least one server" : undefined}
                disabled={isPending || last}
              >
                <TrashIcon />
              </Button>
            }
            title={`Remove ${name} from ${network.name}?`}
            description={`${name} restarts and accepts players directly again. ${disconnects}`}
            action="Remove server"
            destructive
            onConfirm={() =>
              run(
                { action: "remove", serverId: backend.serverId },
                { loading: `Removing ${name}…`, success: `Removed ${name} from ${network.name}` },
              )
            }
          />
        </div>
      </TableCell>
    </TableRow>
  )
}
