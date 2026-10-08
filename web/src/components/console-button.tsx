import type { Icon } from "@phosphor-icons/react"
import type { ComponentProps } from "react"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

/** A button of the toolbar of the console or the terminal with only an icon, named by its label and tooltip. */
export function ConsoleButton({ icon: Icon, label, ...props }: { icon: Icon; label: string } & ComponentProps<"button">) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={label}
          {...props}
          className="grid size-8 shrink-0 place-items-center rounded-lg text-console-muted transition-colors hover:bg-console-overlay/10 hover:text-console-foreground focus-visible:outline-2 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-40 aria-pressed:bg-console-warn/15 aria-pressed:text-console-warn"
        >
          <Icon className="size-4" />
        </button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}
