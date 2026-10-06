import { ArrowsClockwiseIcon, LinkIcon, PencilSimpleIcon, ShieldCheckIcon, SignOutIcon, WarningIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { Trans } from "react-i18next"
import { toast } from "sonner"
import { Callout, ErrorCallout } from "@/components/callout"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Section } from "@/components/section"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { type Node, nodesQuery } from "@/features/nodes/api"
import { useOperation } from "@/features/operations/use-operation"
import { formatAgo, formatBytes, formatDate } from "@/lib/format"
import {
  type NodeOverlay,
  nodeOverlayQuery,
  overlayQuery,
  useJoinOverlay,
  useLeaveOverlay,
  useRotateOverlayKey,
  useSetOverlayEndpoint,
} from "./api"

/** Whether peers had no handshake for 5 minutes, e.g. as a firewall blocks the port, which keepalives renew every 2. */
const stale = (handshake?: string) => !handshake || Date.now() - Date.parse(handshake) > 5 * 60_000

/** A node's part in the private network of the nodes: whether it may and can join, its address and its peers. */
export function OverlaySection({ node }: { node: Node }) {
  const manage = useAccess().can("overlay.manage")
  const { data, error } = useQuery(nodeOverlayQuery(node.id))
  return (
    <Section
      title={t("Private network")}
      description={t("Proxies reach the servers of other members over WireGuard, at ports that only they reach.")}
      actions={data?.member && manage && <MemberActions node={node} overlay={data} />}
    >
      {error ? (
        <ErrorCallout error={error} />
      ) : !data ? (
        <Skeleton className="h-32 rounded-xl" />
      ) : !data.allowed ? (
        <Callout icon={ShieldCheckIcon} title={t("Not allowed on this node")}>
          <Trans
            i18nKey="Its administrator allows it with <command/> on the node."
            components={{ command: <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">noryx-agent overlay allow</code> }}
          />
        </Callout>
      ) : data.unsupported ? (
        <Callout tone="warning" icon={WarningIcon} title={t("{{name}} can't join", { name: node.name })}>
          {data.unsupported}
        </Callout>
      ) : data.member ? (
        <Membership overlay={data} />
      ) : (
        <JoinForm node={node} manage={manage} />
      )}
    </Section>
  )
}

/** Lets a node join, at the address where the other nodes reach it. */
function JoinForm({ node, manage }: { node: Node; manage: boolean }) {
  const [endpoint, setEndpoint] = useState("")
  const join = useJoinOverlay(node.id)
  const { data: overlay } = useQuery(overlayQuery)
  const host = node.address?.slice(0, node.address.lastIndexOf(":"))

  function submit(event: FormEvent) {
    event.preventDefault()
    join.mutate(endpoint.trim(), { onSuccess: (m) => toast.success(t("{{name}} joined as {{address}}", { name: node.name, address: m.address })) })
  }

  return (
    <form onSubmit={submit} className="surface grid gap-4 rounded-xl p-5">
      <p className="text-sm text-muted-foreground">
        {t("Not part of it yet. Joining restarts nothing: networks move to it when they are applied again.")}
      </p>
      {manage && (
        <div className="flex flex-wrap items-end gap-3">
          <Field className="min-w-60 flex-1">
            <FieldLabel htmlFor="overlay-endpoint">{t("Endpoint")}</FieldLabel>
            <Input
              id="overlay-endpoint"
              className="font-mono"
              value={endpoint}
              placeholder={host && overlay ? `${host}:${overlay.port}` : "203.0.113.10:51820"}
              onChange={(e) => setEndpoint(e.target.value)}
            />
            <FieldDescription>{t("Where the other nodes reach it; empty for the host of its agent's address.")}</FieldDescription>
          </Field>
          <Button type="submit" disabled={join.isPending} className="mb-6">
            <LinkIcon />
            {join.isPending ? t("Joining…") : t("Join")}
          </Button>
        </div>
      )}
      {join.error && <FieldError>{join.error.message}</FieldError>}
    </form>
  )
}

/** The address and key of a member, and how it reaches the others. */
function Membership({ overlay }: { overlay: NodeOverlay }) {
  const { data: nodes } = useQuery(nodesQuery)
  const member = overlay.member!
  const facts = [
    [t("Address"), member.address],
    [t("Endpoint"), overlay.endpoint ?? "–"],
    [t("Key"), overlay.fingerprint ? `${overlay.fingerprint}…` : "–"],
    [t("Joined"), formatDate(member.joinedAt)],
  ]
  return (
    <div className="space-y-4">
      {member.problem && (
        <Callout tone="warning" icon={WarningIcon} role="alert" title={t("Not configured")}>
          {member.problem}
        </Callout>
      )}
      <dl className="surface grid grid-cols-2 gap-x-6 gap-y-4 rounded-xl px-5 py-4 md:grid-cols-4">
        {facts.map(([term, value]) => (
          <div key={term} className="min-w-0">
            <dt className="text-xs text-muted-foreground">{term}</dt>
            <dd className="mt-0.5 truncate font-mono text-sm" title={value}>
              {value}
            </dd>
          </div>
        ))}
      </dl>
      {overlay.peers.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("No other node is part of it yet.")}</p>
      ) : (
        <div className="surface overflow-x-auto rounded-xl">
          <table className="w-full text-sm">
            <thead className="text-left text-xs text-muted-foreground">
              <tr className="border-b">
                <th className="px-4 py-2.5 font-medium">{t("Node")}</th>
                <th className="px-4 py-2.5 font-medium">{t("Address")}</th>
                <th className="px-4 py-2.5 font-medium">{t("Endpoint")}</th>
                <th className="px-4 py-2.5 font-medium">{t("Latest handshake")}</th>
                <th className="px-4 py-2.5 text-right font-medium">{t("Received / sent")}</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {overlay.peers.map((p) => (
                <tr key={p.nodeId}>
                  <td className="px-4 py-2.5 font-medium">{nodes?.find((n) => n.id === p.nodeId)?.name ?? p.nodeId}</td>
                  <td className="px-4 py-2.5 font-mono text-xs">{p.address}</td>
                  <td className="px-4 py-2.5 font-mono text-xs">{p.endpoint ?? "–"}</td>
                  <td className="px-4 py-2.5">
                    {stale(p.latestHandshake) ? (
                      <Pill tone="warning">{p.latestHandshake ? formatAgo(p.latestHandshake) : t("never")}</Pill>
                    ) : (
                      formatAgo(p.latestHandshake!)
                    )}
                  </td>
                  <td className="px-4 py-2.5 text-right tabular-nums">
                    {formatBytes(p.receivedBytes)} / {formatBytes(p.sentBytes)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

/** Changes where the others reach a member, rotates its key, or removes it. */
function MemberActions({ node, overlay }: { node: Node; overlay: NodeOverlay }) {
  const [editing, setEditing] = useState(false)
  const rotate = useRotateOverlayKey(node.id)
  const leave = useLeaveOverlay(node.id)
  const operation = useOperation()
  return (
    <div className="flex flex-wrap gap-2">
      <Button variant="outline" onClick={() => setEditing(true)}>
        <PencilSimpleIcon />
        {t("Endpoint")}
      </Button>
      <ConfirmDialog
        trigger={
          <Button variant="outline" disabled={rotate.isPending}>
            <ArrowsClockwiseIcon />
            {t("Rotate key")}
          </Button>
        }
        title={t("Rotate the key of {{name}}?", { name: node.name })}
        description={t("The node creates a new key, and the others get its public key. Until then, which takes a few seconds, they don't reach it.")}
        action={t("Rotate key")}
        onConfirm={() =>
          rotate.mutate(undefined, {
            onSuccess: () => toast.success(t("Rotated the key of {{name}}", { name: node.name })),
            onError: (e) => toast.error(e.message),
          })
        }
      />
      <ConfirmDialog
        trigger={
          <Button variant="outline" className="text-destructive hover:bg-destructive/10 hover:text-destructive" disabled={leave.isPending}>
            <SignOutIcon />
            {t("Remove")}
          </Button>
        }
        title={t("Remove {{name}} from the private network?", { name: node.name })}
        description={t("Networks then reach its servers, and it reaches other servers, over public ports again: these servers restart once.")}
        action={t("Remove")}
        destructive
        onConfirm={() =>
          operation.run((onStart) => leave.mutateAsync({ onStart }), {
            title: t("Removing {{name}} from the private network…", { name: node.name }),
            notify: true,
            done: () => ({ message: t("Removed {{name}} from the private network", { name: node.name }) }),
          })
        }
      />
      {editing && <EndpointDialog node={node} current={overlay.member?.endpoint ?? ""} onClose={() => setEditing(false)} />}
    </div>
  )
}

function EndpointDialog({ node, current, onClose }: { node: Node; current: string; onClose: () => void }) {
  const [endpoint, setEndpoint] = useState(current)
  const save = useSetOverlayEndpoint(node.id)

  function submit(event: FormEvent) {
    event.preventDefault()
    save.mutate(endpoint.trim(), { onSuccess: onClose })
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{t("Endpoint of {{name}}", { name: node.name })}</DialogTitle>
            <DialogDescription>{t("Where the other nodes reach it, as host:port; empty for the host of its agent's address.")}</DialogDescription>
          </DialogHeader>
          <Field>
            <FieldLabel htmlFor="overlay-endpoint-edit">{t("Endpoint")}</FieldLabel>
            <Input id="overlay-endpoint-edit" autoFocus className="font-mono" value={endpoint} onChange={(e) => setEndpoint(e.target.value)} />
            {save.error && <FieldError>{save.error.message}</FieldError>}
          </Field>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={save.isPending}>
              {t("Save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
