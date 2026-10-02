import { KeyIcon, ShieldCheckIcon, TrashIcon } from "@phosphor-icons/react"
import { useNavigate } from "@tanstack/react-router"
import { useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { formatDate } from "@/lib/format"
import { type Node, useDeleteNode, useNewJoinToken, useRenewCertificate } from "./api"
import { EnrollSteps } from "./enroll-steps"

/** Issues a new join token, e.g. for a node whose agent was reinstalled. */
export function NewJoinTokenButton({ node, variant = "outline" }: { node: Node; variant?: "outline" | "default" }) {
  const issue = useNewJoinToken(node.id)
  const [open, setOpen] = useState(false)

  return (
    <>
      <Button
        variant={variant}
        disabled={issue.isPending}
        onClick={() => issue.mutate(undefined, { onSuccess: () => setOpen(true), onError: (e) => toast.error(e.message) })}
      >
        <KeyIcon />
        New join token
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Connect {node.name}</DialogTitle>
            <DialogDescription>Any earlier join token of this node no longer works.</DialogDescription>
          </DialogHeader>
          {issue.data && <EnrollSteps token={issue.data} />}
          <DialogFooter>
            <DialogClose asChild>
              <Button>Done</Button>
            </DialogClose>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

export function RemoveNodeButton({ node }: { node: Node }) {
  const remove = useDeleteNode()
  const navigate = useNavigate()

  function confirm() {
    remove.mutate(node.id, {
      onSuccess: () => {
        toast.success(`Removed ${node.name}`)
        navigate({ to: "/nodes" })
      },
      onError: (e) => toast.error(e.message),
    })
  }

  return (
    <ConfirmDialog
      trigger={
        <Button variant="destructive">
          <TrashIcon />
          Remove node
        </Button>
      }
      title={`Remove ${node.name}?`}
      description="The panel stops managing this node. Its servers keep running until you stop them on the node or uninstall the agent."
      action="Remove node"
      destructive
      onConfirm={confirm}
    />
  )
}

/** Renews the node certificate right away, e.g. when its key may have leaked. */
export function RenewCertificateButton({ node }: { node: Node }) {
  const renew = useRenewCertificate(node.id)
  return (
    <Button
      variant="outline"
      disabled={renew.isPending}
      onClick={() =>
        renew.mutate(undefined, {
          onSuccess: ({ certificateExpiresAt }) =>
            toast.success(`Renewed the certificate of ${node.name}`, { description: `Valid until ${formatDate(certificateExpiresAt)}` }),
          onError: (e) => toast.error(e.message),
        })
      }
    >
      <ShieldCheckIcon />
      {renew.isPending ? "Renewing…" : "Renew certificate"}
    </Button>
  )
}
