import { ArrowLeftIcon, ArrowSquareOutIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { Button } from "@/components/ui/button"
import { DialogClose, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import type { Operation } from "./api"
import { CancelButton } from "./cancel-button"
import { OperationProgress } from "./operation-progress"

/**
 * What a dialog shows once its action runs as an operation: its progress, and that it can go
 * on in the background or be cancelled, if it can stop safely. If it failed or was cancelled,
 * the form opens again with what was entered.
 */
export function OperationStatus({
  op,
  title,
  onBackground,
  onBack,
}: {
  op: Operation
  title: string
  onBackground: () => void
  onBack: () => void
}) {
  return (
    <div className="grid gap-6">
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
        <DialogDescription>
          {op.error
            ? op.cancelled
              ? t("It was cancelled at the marked step.")
              : t("It failed at the marked step.")
            : t("This takes a moment. It keeps running in the background too, and a notification tells you how it went.")}
        </DialogDescription>
      </DialogHeader>
      <OperationProgress op={op} />
      <DialogFooter>
        {op.error ? (
          <>
            <Button variant="outline" onClick={onBack}>
              <ArrowLeftIcon />
              {t("Back")}
            </Button>
            <DialogClose asChild>
              <Button>{t("Close")}</Button>
            </DialogClose>
          </>
        ) : (
          <>
            <CancelButton op={op} />
            <Button variant="outline" disabled={!!op.finishedAt} onClick={onBackground}>
              <ArrowSquareOutIcon />
              {t("Continue in the background")}
            </Button>
          </>
        )}
      </DialogFooter>
    </div>
  )
}
