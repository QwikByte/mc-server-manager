import { cn } from "@/lib/utils"

export type LampState = "on" | "starting" | "off" | "unset"

const panes = [
  [1, 1],
  [4, 1],
  [1, 4],
  [4, 4],
]

/**
 * A redstone lamp in pixel art: the status indicator for nodes and servers.
 * It is lit while something runs, flickers while it starts and stays dark otherwise.
 * Without a label the lamp is decorative and hidden from assistive technology.
 */
export function Lamp({ state, label, className }: { state: LampState; label?: string; className?: string }) {
  const lit = state === "on" || state === "starting"
  return (
    <svg
      viewBox="0 0 7 7"
      role={label ? "img" : undefined}
      aria-label={label}
      aria-hidden={!label}
      shapeRendering="crispEdges"
      className={cn(
        "size-3.5 shrink-0",
        lit && "drop-shadow-[0_0_5px_var(--lamp-glow)]",
        state === "starting" && "motion-safe:animate-lamp-flicker",
        state === "unset" && "opacity-35",
        className,
      )}
    >
      {label && <title>{label}</title>}
      <rect width="7" height="7" fill={lit ? "var(--lamp-on-frame)" : "var(--lamp-off-frame)"} />
      {panes.map(([x, y]) => (
        <rect key={`${x}-${y}`} x={x} y={y} width="2" height="2" fill={lit ? "var(--lamp-on)" : "var(--lamp-off)"} />
      ))}
    </svg>
  )
}
