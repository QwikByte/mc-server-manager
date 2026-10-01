import { cn } from "@/lib/utils"

/** A block on an emerald tile; the same drawing is the favicon. */
export function Logo({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" aria-hidden className={cn("size-9 shrink-0 drop-shadow-[0_4px_12px_rgb(16_185_129/0.35)]", className)}>
      <defs>
        <linearGradient id="logo-tile" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#34d399" />
          <stop offset="1" stopColor="#0d9488" />
        </linearGradient>
      </defs>
      <rect width="32" height="32" rx="9" fill="url(#logo-tile)" />
      <path d="M16 6.5 25 11.5 16 16.5 7 11.5Z" fill="#fff" />
      <path d="M7 11.5 16 16.5V26.5L7 21.5Z" fill="#fff" fillOpacity=".72" />
      <path d="M25 11.5 16 16.5V26.5L25 21.5Z" fill="#fff" fillOpacity=".5" />
    </svg>
  )
}
