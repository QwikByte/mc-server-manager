import { t } from "i18next"
import { type ReactElement, type ReactNode, type RefObject, useId, useRef, useState } from "react"
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
import { Field, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"

interface Confirmation {
  title: ReactNode
  description: ReactNode
  action: string
  destructive?: boolean
  /** Has to be typed before the action, e.g. the name of a server that is deleted with all its worlds. */
  confirmText?: string
  onConfirm: () => void
}

/**
 * Asks for confirmation before an action that can't be undone or disrupts players. It
 * opens through its trigger, or is controlled with open and onOpenChange.
 */
export function ConfirmDialog({
  trigger,
  open,
  onOpenChange,
  ...confirmation
}: Confirmation & {
  trigger?: ReactElement
  open?: boolean
  onOpenChange?: (open: boolean) => void
}) {
  const field = useRef<HTMLInputElement>(null)
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      {trigger && <AlertDialogTrigger asChild>{trigger}</AlertDialogTrigger>}
      <AlertDialogContent
        // Focus starts in the text to type rather than on Cancel.
        onOpenAutoFocus={(e) => {
          if (confirmation.confirmText === undefined) return
          e.preventDefault()
          field.current?.focus()
        }}
      >
        <Content {...confirmation} field={field} />
      </AlertDialogContent>
    </AlertDialog>
  )
}

/**
 * The content, created anew each time the dialog opens. It confirms once: the button stays
 * clickable while the dialog closes, so a double click would run the action twice.
 */
function Content({
  title,
  description,
  action,
  destructive = false,
  confirmText,
  onConfirm,
  field,
}: Confirmation & { field: RefObject<HTMLInputElement | null> }) {
  const confirmed = useRef(false)
  const button = useRef<HTMLButtonElement>(null)
  const [typed, setTyped] = useState("")
  const id = useId()
  const ready = confirmText === undefined || typed === confirmText
  return (
    <>
      <AlertDialogHeader>
        <AlertDialogTitle>{title}</AlertDialogTitle>
        <AlertDialogDescription>{description}</AlertDialogDescription>
      </AlertDialogHeader>
      {confirmText !== undefined && (
        <Field>
          <FieldLabel htmlFor={id}>{t("Type {{name}} to confirm", { name: confirmText })}</FieldLabel>
          <Input
            id={id}
            ref={field}
            autoComplete="off"
            spellCheck={false}
            className="font-mono"
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && ready && button.current?.click()}
          />
        </Field>
      )}
      <AlertDialogFooter>
        <AlertDialogCancel>{t("Cancel")}</AlertDialogCancel>
        <AlertDialogAction
          ref={button}
          variant={destructive ? "destructive" : "default"}
          disabled={!ready}
          onClick={() => {
            if (confirmed.current) return
            confirmed.current = true
            onConfirm()
          }}
        >
          {action}
        </AlertDialogAction>
      </AlertDialogFooter>
    </>
  )
}
