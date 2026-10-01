import { CaretLeftIcon } from "@phosphor-icons/react"
import { createLink } from "@tanstack/react-router"
import type { ComponentProps } from "react"

/** Leads back to the page one level up. */
export const BackLink = createLink(function BackLink({ children, ...props }: ComponentProps<"a">) {
  return (
    <a
      {...props}
      className="group mb-5 inline-flex items-center gap-1.5 rounded-full py-1 pr-3 pl-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
    >
      <CaretLeftIcon className="size-3.5 transition-transform group-hover:-translate-x-0.5" weight="bold" />
      {children}
    </a>
  )
})
