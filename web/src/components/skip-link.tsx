import { t } from "i18next"
import { content, contentId } from "@/components/page-focus"
import { buttonVariants } from "@/components/ui/button"
import { cn } from "@/lib/utils"

/** The first stop of the keyboard, shown while focused, which skips the navigation to the content. */
export function SkipLink() {
  return (
    <a
      href={`#${contentId}`}
      // Without changing the address, which the router would take for navigating.
      onClick={(event) => {
        event.preventDefault()
        content()?.focus()
      }}
      className={cn(buttonVariants(), "fixed top-3 left-3 z-50 shadow-lg transition-none not-focus:sr-only")}
    >
      {t("Skip to content")}
    </a>
  )
}
