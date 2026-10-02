import type { ReactNode } from "react"
import { Logo } from "@/components/logo"
import { ThemeToggle } from "@/components/theme-toggle"

/** The frame of the pages before signing in: the logo, a title and a card. */
export function AuthLayout({ title, description, children }: { title: string; description: ReactNode; children: ReactNode }) {
  return (
    <main className="relative grid min-h-svh place-items-center overflow-hidden px-4 py-16">
      <div aria-hidden className="pointer-events-none absolute inset-0 bg-blocks" />
      <ThemeToggle className="absolute top-4 right-4 w-28" />
      <div className="relative w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-5 text-center">
          <Logo className="size-14" />
          <div className="space-y-1.5">
            <h1 className="heading text-2xl">{title}</h1>
            <p className="text-sm text-muted-foreground">{description}</p>
          </div>
        </div>
        <div className="rounded-2xl bg-card/80 p-6 shadow-xl ring-1 ring-foreground/8 backdrop-blur-xl dark:shadow-black/40">
          {children}
        </div>
      </div>
    </main>
  )
}
