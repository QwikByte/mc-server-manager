import { Link } from "@tanstack/react-router"
import { useState } from "react"
import { cn } from "@/lib/utils"
import { faceUrl } from "./api"

const sizes = { sm: "size-7 text-xs", md: "size-8 text-xs", lg: "size-14 text-xl" }

/**
 * A player's face, which the master cuts from their skin, over the first letter of their name, which shows while it
 * loads and for players without a face.
 */
export function PlayerAvatar({ name, size = "sm", className }: { name: string; size?: keyof typeof sizes; className?: string }) {
  const [failed, setFailed] = useState<string>()
  return (
    <span
      aria-hidden
      className={cn("relative grid shrink-0 place-items-center overflow-hidden rounded-lg bg-primary/10 font-semibold text-primary", sizes[size], className)}
    >
      {name.replace(/^\./, "")[0]?.toUpperCase()}
      {failed !== name && (
        <img
          src={faceUrl(name)}
          alt=""
          loading="lazy"
          onError={() => setFailed(name)}
          className="absolute inset-0 size-full [image-rendering:pixelated]"
        />
      )}
    </span>
  )
}

/** A player's name with their face, which opens their page. */
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
