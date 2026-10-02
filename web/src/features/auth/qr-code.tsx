import { useMemo } from "react"
import { encode } from "uqr"
import { cn } from "@/lib/utils"

/** A QR code, black on white in both themes so that cameras read it. */
export function QrCode({ value, label, className }: { value: string; label: string; className?: string }) {
  const { size, path } = useMemo(() => {
    const qr = encode(value, { border: 2 })
    const path = qr.data.flatMap((row, y) => row.map((dark, x) => (dark ? `M${x} ${y}h1v1h-1z` : ""))).join("")
    return { size: qr.size, path }
  }, [value])
  return (
    <svg role="img" aria-label={label} viewBox={`0 0 ${size} ${size}`} shapeRendering="crispEdges" className={cn("rounded-xl bg-white", className)}>
      <path d={path} fill="#000" />
    </svg>
  )
}
