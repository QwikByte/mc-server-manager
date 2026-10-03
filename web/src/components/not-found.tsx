import { HouseIcon, SignpostIcon } from "@phosphor-icons/react"
import { Link, useLocation } from "@tanstack/react-router"
import { EmptyState } from "@/components/empty-state"
import { Button } from "@/components/ui/button"

/** Shown within the panel for addresses that lead to no page. */
export function NotFound() {
  const { pathname } = useLocation()
  return (
    <EmptyState
      icon={SignpostIcon}
      tone="warning"
      title="Page not found"
      description={
        <>
          The panel has no page at <span className="font-mono break-all">{pathname}</span>. It may have moved, or the link is mistyped.
        </>
      }
    >
      <Button asChild>
        <Link to="/">
          <HouseIcon />
          Go to the start page
        </Link>
      </Button>
    </EmptyState>
  )
}
