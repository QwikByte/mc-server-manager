import { createLink } from "@tanstack/react-router"
import { motion } from "motion/react"
import type { ComponentProps } from "react"
import { cn } from "@/lib/utils"

const tabClass = "relative isolate flex items-center gap-2 rounded-lg px-3.5 py-1.5 text-sm font-medium whitespace-nowrap text-muted-foreground"

/** A link in Tabs, highlighted while its route is active; the highlight glides to the next tab. */
export const TabLink = createLink(function TabLink({
  className,
  children,
  ...props
}: ComponentProps<"a"> & { "data-status"?: string }) {
  return (
    <a {...props} className={cn(tabClass, "transition-colors hover:text-foreground data-[status=active]:text-foreground", className)}>
      {props["data-status"] === "active" && (
        <motion.span
          layoutId="tab"
          aria-hidden
          className="absolute inset-0 -z-10 rounded-lg bg-card shadow-sm"
          transition={{ type: "spring", bounce: 0.15, duration: 0.4 }}
        />
      )}
      {children}
    </a>
  )
})
