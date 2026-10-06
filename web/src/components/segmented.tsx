import { LayoutGroup, motion } from "motion/react"
import { useId } from "react"
import { cn } from "@/lib/utils"

/** A choice among a few options in a row, e.g. the time range of charts; the highlight glides to the choice. */
export function Segmented<T extends string>({
  label,
  value,
  options,
  onChange,
  className,
}: {
  label: string
  value: T
  options: { value: T; label: string }[]
  onChange: (value: T) => void
  className?: string
}) {
  return (
    <div role="radiogroup" aria-label={label} className={cn("inline-flex rounded-lg bg-muted p-0.5", className)}>
      <LayoutGroup id={useId()}>
        {options.map((option) => (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={value === option.value}
            onClick={() => onChange(option.value)}
            className="relative isolate h-7 rounded-md px-3 text-xs font-medium text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring aria-checked:text-foreground"
          >
            {value === option.value && <Highlight className="rounded-md" />}
            {option.label}
          </button>
        ))}
      </LayoutGroup>
    </div>
  )
}

/** The raised surface under the chosen option of a group, which glides to the next choice. */
export function Highlight({ className }: { className?: string }) {
  return (
    <motion.span
      layoutId="highlight"
      aria-hidden
      className={cn("absolute inset-0 -z-10 bg-card shadow-sm", className)}
      transition={{ type: "spring", bounce: 0.15, duration: 0.4 }}
    />
  )
}
