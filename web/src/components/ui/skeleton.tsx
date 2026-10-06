import { cn } from "cn"

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      className={cn("relative overflow-hidden rounded-md bg-muted after:absolute after:inset-0 after:animate-shimmer after:bg-linear-to-r after:from-transparent after:via-foreground/6 after:to-transparent motion-reduce:animate-pulse motion-reduce:after:hidden", className)}
      {...props}
    />
  )
}

export { Skeleton }
