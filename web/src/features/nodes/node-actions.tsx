import { DotsThreeIcon, KeyIcon, ShieldCheckIcon, TrashIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { useAccess } from "@/features/access/use-access"
import { formatDate } from "@/lib/format"
import { type Node, useNewJoinToken, useRenewCertificate } from "./api"
import { EnrollSteps } from "./enroll-steps"
import { RemoveNodeDialog } from "./remove-node-dialog"

/** Issues a new join token, e.g. for a node whose agent was reinstalled, and shows how to connect with it. */
function useJoinToken(node: Node) {
  const issue = useNewJoinToken(node.id)
  const [open, setOpen] = useState(false)
  const dialog = (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("Connect {{name}}", { name: node.name })}</DialogTitle>
          <DialogDescription>{t("Any earlier join token of this node no longer works.")}</DialogDescription>
        </DialogHeader>
        {issue.data && <EnrollSteps token={issue.data} />}
        <DialogFooter>
          <DialogClose asChild>
            <Button>{t("Done")}</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
  const start = () => issue.mutate(undefined, { onSuccess: () => setOpen(true), onError: (e) => toast.error(e.message) })
  return { start, pending: issue.isPending, dialog }
}

export function NewJoinTokenButton({ node }: { node: Node }) {
  const token = useJoinToken(node)
  return (
    <>
      <Button disabled={token.pending} onClick={token.start}>
        <KeyIcon />
        {t("New join token")}
      </Button>
      {token.dialog}
    </>
  )
}

/** The rarer actions on a node behind a menu: renew its certificate, issue a join token and remove it. */
export function NodeMenu({ node }: { node: Node }) {
  const { can } = useAccess()
  const token = useJoinToken(node)
  const renew = useRenewCertificate(node.id)
  const [removing, setRemoving] = useState(false)
  // Renewing the certificate right away helps e.g. when its key may have leaked.
  const renewable = node.status === "online" && can("nodes.certificates", node.id)
  const tokens = !!node.enrolledAt && can("nodes.enroll")
  const removable = can("nodes.delete", node.id)
  if (!renewable && !tokens && !removable) return null

  function renewNow() {
    renew.mutate(undefined, {
      onSuccess: ({ certificateExpiresAt }) =>
        toast.success(t("Renewed the certificate of {{name}}", { name: node.name }), {
          description: t("Valid until {{date}}", { date: formatDate(certificateExpiresAt) }),
        }),
      onError: (e) => toast.error(e.message),
    })
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="icon" aria-label={t("More actions for {{name}}", { name: node.name })} title={t("More actions")}>
            <DotsThreeIcon weight="bold" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-56">
          {renewable && (
            <DropdownMenuItem disabled={renew.isPending} onSelect={renewNow}>
              <ShieldCheckIcon />
              {t("Renew certificate")}
            </DropdownMenuItem>
          )}
          {tokens && (
            <DropdownMenuItem disabled={token.pending} onSelect={token.start}>
              <KeyIcon />
              {t("New join token")}
            </DropdownMenuItem>
          )}
          {removable && (
            <>
              {(renewable || tokens) && <DropdownMenuSeparator />}
              <DropdownMenuItem variant="destructive" onSelect={() => setRemoving(true)}>
                <TrashIcon />
                {t("Remove node…")}
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      {token.dialog}
      {removing && <RemoveNodeDialog node={node} onClose={() => setRemoving(false)} />}
    </>
  )
}
