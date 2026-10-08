import { formatAgo, formatAgoTitle } from "@/lib/format"

/** When something was or will be, as the user wants times, with the other way in its tooltip. */
export function Ago({ time, now, className }: { time: string; now?: number; className?: string }) {
  return (
    <time dateTime={time} title={formatAgoTitle(time, now)} className={className}>
      {formatAgo(time, now)}
    </time>
  )
}
