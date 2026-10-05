import { t } from "i18next"
import { type ReactElement, type ReactNode, useRef } from "react"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"

/**
 * Asks for confirmation before an action that can't be undone or disrupts players. It
 * opens through its trigger, or is controlled with open and onOpenChange.
 */
export function ConfirmDialog({
  trigger,
  open,
  onOpenChange,
  title,
  description,
  action,
  destructive = false,
  onConfirm,
}: {
  trigger?: ReactElement
  open?: boolean
  onOpenChange?: (open: boolean) => void
  title: ReactNode
  description: ReactNode
  action: string
  destructive?: boolean
  onConfirm: () => void
}) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      {trigger && <AlertDialogTrigger asChild>{trigger}</AlertDialogTrigger>}
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t("Cancel")}</AlertDialogCancel>
          <ConfirmAction destructive={destructive} onConfirm={onConfirm}>
            {action}
          </ConfirmAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

/**
 * Confirms once: the button stays clickable while the dialog closes, so a double click
 * would run the action twice. The content is created anew each time the dialog opens.
 */
function ConfirmAction({ destructive, onConfirm, children }: { destructive: boolean; onConfirm: () => void; children: ReactNode }) {
  const confirmed = useRef(false)
  return (
    <AlertDialogAction
      variant={destructive ? "destructive" : "default"}
      onClick={() => {
        if (confirmed.current) return
        confirmed.current = true
        onConfirm()
      }}
    >
      {children}
    </AlertDialogAction>
  )
}
