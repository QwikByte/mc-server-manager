import { createLink } from "@tanstack/react-router"
import type { ComponentProps } from "react"
import { cn } from "@/lib/utils"

export const tabClass = "flex items-center gap-2 rounded-lg px-3.5 py-1.5 text-sm font-medium whitespace-nowrap text-muted-foreground"

/** A link in Tabs, highlighted while its route is active. */
export const TabLink = createLink(function TabLink({ className, ...props }: ComponentProps<"a">) {
  return (
    <a
      {...props}
      className={cn(
        tabClass,
        "transition-colors hover:text-foreground data-[status=active]:bg-card data-[status=active]:text-foreground data-[status=active]:shadow-sm",
        className,
      )}
    />
  )
})
