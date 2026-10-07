import { ArrowClockwiseIcon, HouseIcon, WarningOctagonIcon } from "@phosphor-icons/react"
import { type ErrorComponentProps, Link } from "@tanstack/react-router"
import { t } from "i18next"
import { EmptyState } from "@/components/empty-state"
import { Button } from "@/components/ui/button"

/** Shown in place of a page that failed to render, within the panel if it can be. */
export function ErrorPage({ error }: ErrorComponentProps) {
  return (
    <EmptyState
      icon={WarningOctagonIcon}
      tone="destructive"
      title={t("This page couldn't be shown")}
      description={<span className="break-words">{error instanceof Error ? error.message : String(error)}</span>}
    >
      <div className="flex flex-wrap justify-center gap-2">
        <Button onClick={() => location.reload()}>
          <ArrowClockwiseIcon />
          {t("Reload")}
        </Button>
        <Button variant="outline" asChild>
          <Link to="/">
            <HouseIcon />
            {t("Back to the overview")}
          </Link>
        </Button>
      </div>
    </EmptyState>
  )
}
