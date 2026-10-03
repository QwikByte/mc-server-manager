import { TruckIcon, WarningCircleIcon, WarningIcon } from "@phosphor-icons/react"
import { useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Callout } from "@/components/callout"
import { Button } from "@/components/ui/button"
import { Progress } from "@/components/ui/progress"
import { useServerUsage } from "@/features/usage/api"
import { formatBytes } from "@/lib/format"
import { type Move, useMove } from "./api"

/**
 * How the move of a server goes: its progress, why it failed, or what went wrong after it
 * moved. When it is done, the page of the server on its old node continues on the new one.
 */
export function MoveStatus({ nodeId, serverId }: { nodeId: string; serverId: string }) {
  const move = useMove(serverId)
  const navigate = useNavigate()
  const [dismissed, setDismissed] = useState<string>()
  // The node the server moved to from this one.
  const movedTo = move?.phase === "done" && move.from === nodeId ? move.to : undefined
  const [name, toName] = [move?.serverName, move?.toName]

  useEffect(() => {
    if (!movedTo) return
    toast.success(t("Moved {{name}} to {{node}}", { name, node: toName }))
    void navigate({ to: "/nodes/$nodeId/servers/$serverId", params: { nodeId: movedTo, serverId }, replace: true })
  }, [movedTo, name, toName, navigate, serverId])

  if (!move || movedTo || dismissed === move.startedAt) return null
  const dismiss = (
    <Button size="sm" variant="ghost" className="-my-1" onClick={() => setDismissed(move.startedAt)}>
      {t("Dismiss")}
    </Button>
  )
  if (!move.finishedAt) return <Progressing move={move} />
  if (move.phase === "failed" && move.from === nodeId) {
    return (
      <Callout
        tone="destructive"
        icon={WarningCircleIcon}
        role="alert"
        title={t("Moving to {{node}} failed", { node: move.toName })}
        className="mb-6"
      >
        <p>
          {move.error} {t("The server stayed on this node, and runs again if it ran before.")}
        </p>
        <div className="mt-2">{dismiss}</div>
      </Callout>
    )
  }
  if (move.phase === "done" && move.to === nodeId && move.warnings.length > 0) {
    return (
      <Callout tone="warning" icon={WarningIcon} title={t("The server moved here, but not everything went well")} className="mb-6">
        <ul className="list-disc space-y-1 pl-4">
          {move.warnings.map((w) => (
            <li key={w}>{w}</li>
          ))}
        </ul>
        <div className="mt-2">{dismiss}</div>
      </Callout>
    )
  }
  return null
}

function Progressing({ move }: { move: Move }) {
  // The size of the data is an estimate for the archive, which is compressed a little.
  const { usage } = useServerUsage(move.from, move.serverId)
  const total = usage?.diskBytes
  const copying = move.phase === "copying"
  const progress = copying
    ? total
      ? t("Copying its files: {{copied}} of about {{total}}…", { copied: formatBytes(move.bytes), total: formatBytes(total) })
      : t("Copying its files: {{copied}}…", { copied: formatBytes(move.bytes) })
    : move.phase === "backups"
      ? t("Copying its backups: {{copied}} of {{total}}…", { copied: move.backups, total: move.backupsTotal })
      : move.phase === "stopping"
        ? t("Stopping the server…")
        : t("Starting it on the new node…")
  return (
    <Callout tone="info" icon={TruckIcon} role="status" title={t("Moving to {{node}}", { node: move.toName })} className="mb-6">
      <p>{progress}</p>
      {copying && total ? (
        <Progress value={Math.min(99, (move.bytes / total) * 100)} className="mt-3 h-1.5 max-w-md" aria-label={t("Files copied")} />
      ) : null}
      {move.phase === "backups" && move.backupsTotal > 0 && (
        <Progress value={(move.backups / move.backupsTotal) * 100} className="mt-3 h-1.5 max-w-md" aria-label={t("Backups copied")} />
      )}
    </Callout>
  )
}
