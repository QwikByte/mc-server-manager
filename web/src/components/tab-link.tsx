import { createLink } from "@tanstack/react-router"
import { motion } from "motion/react"
import type { ComponentProps } from "react"
import { cn } from "@/lib/utils"

const tabClass =
  "relative flex h-10 shrink-0 items-center gap-1.5 rounded-t-md px-2.5 text-sm font-medium whitespace-nowrap text-muted-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"

/** A link in Tabs, underlined in the accent while its route is active; the line glides to the next tab. */
export const TabLink = createLink(function TabLink({
  className,
  children,
  ...props
}: ComponentProps<"a"> & { "data-status"?: string }) {
  return (
    <a {...props} className={cn(tabClass, "transition-colors hover:text-foreground data-[status=active]:text-foreground", className)}>
      {children}
      {props["data-status"] === "active" && (
        <motion.span
          layoutId="tab"
          aria-hidden
          className="absolute inset-x-1.5 bottom-0 h-0.5 rounded-full bg-primary"
          transition={{ type: "spring", bounce: 0.15, duration: 0.4 }}
        />
      )}
    </a>
  )
})
