import { LockOpenIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import type { ReactNode } from "react"
import { Trans } from "react-i18next"
import { Callout } from "@/components/callout"
import { LanguageMenu } from "@/components/language-menu"
import { Logo } from "@/components/logo"
import { ThemeToggle } from "@/components/theme-toggle"
import { chosenLanguage } from "@/lib/i18n"

/** The frame of the pages before signing in: the logo, a title and a card. */
export function AuthLayout({ title, description, children }: { title: string; description: ReactNode; children: ReactNode }) {
  return (
    <main className="relative grid min-h-svh place-items-center overflow-hidden px-4 py-16">
      <div aria-hidden className="pointer-events-none absolute inset-0 bg-blocks" />
      <div className="absolute top-4 right-4 flex items-center gap-2">
        <LanguageMenu value={chosenLanguage()} />
        <ThemeToggle className="w-28" />
      </div>
      <div className="relative w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-5 text-center">
          <Logo className="size-14" />
          <div className="space-y-1.5">
            <h1 className="heading text-2xl">{title}</h1>
            <p className="text-sm text-muted-foreground">{description}</p>
          </div>
        </div>
        {!window.isSecureContext && <InsecureNotice />}
        <div className="rounded-2xl bg-card/80 p-6 shadow-xl ring-1 ring-foreground/8 backdrop-blur-xl dark:shadow-black/40">
          {children}
        </div>
      </div>
    </main>
  )
}

/** Browsers drop the secure session cookie over plain HTTP, except at localhost, so signing in can't work here. */
function InsecureNotice() {
  return (
    <Callout tone="warning" icon={LockOpenIcon} role="note" className="mb-4 bg-card/80" title={t("Signing in needs HTTPS")}>
      <Trans
        i18nKey="Browsers only keep the sign-in over HTTPS or at localhost. Sign in through an SSH tunnel instead: run <tunnel/> and open <local/>. Then turn on HTTPS under Settings → General."
        components={{
          tunnel: <span className="font-mono break-all">{`ssh -L 8080:${location.host} <user>@${location.hostname}`}</span>,
          local: <span className="font-mono">http://localhost:8080</span>,
        }}
      />
    </Callout>
  )
}
