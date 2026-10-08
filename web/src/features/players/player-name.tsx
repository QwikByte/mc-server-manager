import { Link } from "@tanstack/react-router"
import { cn } from "@/lib/utils"

const sizes = { sm: "size-7 text-xs", md: "size-8 text-xs", lg: "size-14 text-xl" }

/** A player's picture: the first letter of their name. */
export function PlayerAvatar({ name, size = "sm", className }: { name: string; size?: keyof typeof sizes; className?: string }) {
  return (
    <span
      aria-hidden
      className={cn("grid shrink-0 place-items-center rounded-full bg-primary/10 font-semibold text-primary", sizes[size], className)}
    >
      {name.replace(/^\./, "")[0]?.toUpperCase()}
    </span>
  )
}

/** A player's name with their picture, which opens their page. */
export function PlayerName({ name, size, className }: { name: string; size?: keyof typeof sizes; className?: string }) {
  return (
    <span className={cn("flex min-w-0 items-center gap-3", className)}>
      <PlayerAvatar name={name} size={size} />
      <Link to="/players/$name" params={{ name }} className="truncate font-mono font-medium hover:underline">
        {name}
      </Link>
    </span>
  )
}
