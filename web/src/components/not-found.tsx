import { HouseIcon, SignpostIcon } from "@phosphor-icons/react"
import { Link, useLocation } from "@tanstack/react-router"
import { t } from "i18next"
import { Trans } from "react-i18next"
import { EmptyState } from "@/components/empty-state"
import { Button } from "@/components/ui/button"

/** Shown within the panel for addresses that lead to no page. */
export function NotFound() {
  const { pathname } = useLocation()
  return (
    <EmptyState
      icon={SignpostIcon}
      tone="warning"
      title={t("Page not found")}
      description={
        <Trans
          i18nKey="The panel has no page at <path/>. It may have moved, or the link is mistyped."
          components={{ path: <span className="font-mono break-all">{pathname}</span> }}
        />
      }
    >
      <Button asChild>
        <Link to="/">
          <HouseIcon />
          {t("Go to the start page")}
        </Link>
      </Button>
    </EmptyState>
  )
}
