import { CaretLeftIcon } from "@phosphor-icons/react"
import { createLink } from "@tanstack/react-router"
import type { ComponentProps } from "react"

/** Leads back to the page one level up. */
export const BackLink = createLink(function BackLink({ children, ...props }: ComponentProps<"a">) {
  return (
    <a
      {...props}
      className="group mb-4 inline-flex items-center gap-1 rounded-sm text-[0.8125rem] font-medium text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
    >
      <CaretLeftIcon className="size-3.5 transition-transform group-hover:-translate-x-0.5" weight="bold" />
      {children}
    </a>
  )
})
