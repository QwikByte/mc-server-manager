import { cn } from "@/lib/utils"

/** A bar for how much of something is used; it turns amber and then red as it fills up. */
export function Meter({ value, label, className }: { value: number; label: string; className?: string }) {
  const used = Math.min(Math.max(value, 0), 1)
  return (
    <div
      role="meter"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(used * 100)}
      className={cn("h-1.5 overflow-hidden rounded-full bg-muted", className)}
    >
      <div
        className={cn(
          "h-full rounded-full transition-[width] duration-500",
          used > 0.9 ? "bg-destructive" : used > 0.75 ? "bg-warning" : "bg-primary",
        )}
        style={{ width: `${used * 100}%` }}
      />
    </div>
  )
}
