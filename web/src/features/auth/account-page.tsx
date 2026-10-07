import { ClockIcon, type Icon, KeyIcon, PaletteIcon, ShieldCheckIcon, ShieldIcon, TranslateIcon, UserCircleIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { type ReactNode, useState } from "react"
import { toast } from "sonner"
import { ErrorCallout } from "@/components/callout"
import { FormSection } from "@/components/form-section"
import { IconTile } from "@/components/icon-tile"
import { PageHeader } from "@/components/page-header"
import { Segmented } from "@/components/segmented"
import { StatusBadge } from "@/components/status"
import type { Tone } from "@/components/tone"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { useSettings } from "@/features/preferences/api"
import { type Clock, clock, languageName, languages, msg, timeWith } from "@/lib/i18n"
import { type Theme, useTheme } from "@/lib/theme"
import { meQuery, mfaQuery, useDisableMfa, useEnableMfa, useNewRecoveryCodes, useSetLanguage, type User } from "./api"
import { ConfirmPasswordDialog, MfaSetupDialog, RecoveryCodesDialog } from "./mfa-dialogs"
import { PasswordDialog } from "./password-dialog"

/** The signed-in user's own account: how they sign in, and how the panel looks for them. */
export function AccountPage() {
  const { data: user } = useQuery(meQuery)
  if (!user) return null
  return (
    <>
      <PageHeader
        icon={UserCircleIcon}
        tone="violet"
        title={t("Your account")}
        description={t("Signed in as {{name}}.", { name: user.username })}
      />
      <div className="surface rounded-2xl px-5 sm:px-8 [&>section:last-child]:border-b-0">
        <FormSection title={t("Password")}>
          <AccountRow icon={KeyIcon} tone="info" title={t("Password")} actions={<PasswordDialog username={user.username} />}>
            {t("At least 12 characters that you don't use anywhere else.")}
          </AccountRow>
        </FormSection>
        <FormSection title={t("Two-factor authentication")}>
          <MfaSettings username={user.username} />
        </FormSection>
        <FormSection title={t("Panel")}>
          <PanelSettings user={user} />
        </FormSection>
      </div>
    </>
  )
}

const themes = [
  { value: "light", label: msg("Light") },
  { value: "dark", label: msg("Dark") },
  { value: "system", label: msg("System") },
] satisfies { value: Theme; label: string }[]

// A time in the afternoon shows what each clock means, e.g. 14:30 and 2:30 PM.
const afternoon = new Date(2000, 0, 1, 14, 30)

/** How the panel looks for the user, in all their browsers. */
function PanelSettings({ user }: { user: User }) {
  const { settings, change } = useSettings()
  const theme = useTheme()
  const setLanguage = useSetLanguage()
  return (
    <>
      <AccountRow
        icon={PaletteIcon}
        tone="violet"
        title={t("Colour theme")}
        actions={
          <Segmented
            label={t("Colour theme")}
            value={theme}
            options={themes.map((o) => ({ value: o.value, label: t(o.label) }))}
            onChange={(value) => change({ theme: value })}
          />
        }
      >
        {t("System follows your operating system.")}
      </AccountRow>
      <AccountRow
        icon={ClockIcon}
        tone="info"
        title={t("Time format")}
        actions={
          <Segmented<Clock>
            label={t("Time format")}
            value={settings.clock ?? clock}
            options={(["24h", "12h"] as const).map((value) => ({ value, label: timeWith(value, afternoon) }))}
            onChange={(value) => change({ clock: value })}
          />
        }
      >
        {t("Times show 24 hours, or 12 hours with AM and PM.")}
      </AccountRow>
      <AccountRow
        icon={TranslateIcon}
        tone="info"
        title={t("Language")}
        actions={
          <Select
            value={user.language || "browser"}
            onValueChange={(language) =>
              setLanguage.mutate(language === "browser" ? "" : language, { onError: (error) => toast.error(error.message) })
            }
          >
            <SelectTrigger aria-label={t("Language")} className="w-44">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="browser">{t("Browser language")}</SelectItem>
              {languages.map((code) => (
                <SelectItem key={code} value={code} lang={code}>
                  {languageName(code)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        }
      >
        {t("Dates, times and numbers follow it too.")}
      </AccountRow>
    </>
  )
}

function MfaSettings({ username }: { username: string }) {
  const { data, isPending, error } = useQuery(mfaQuery)
  // The mutations live here, as the dialogs that use them go away when the status changes.
  const enable = useEnableMfa()
  const disable = useDisableMfa()
  const renew = useNewRecoveryCodes()
  const [codes, setCodes] = useState<string[]>()

  if (isPending) return <Skeleton className="h-20 rounded-xl" />
  if (error) return <ErrorCallout error={error} />
  return (
    <>
      {data.enabled ? (
        <AccountRow
          icon={ShieldCheckIcon}
          tone="success"
          title={t("Authenticator app")}
          status={{ tone: "success", label: msg("On") }}
          actions={
            <>
              <ConfirmPasswordDialog
                trigger={<Button variant="outline">{t("New recovery codes")}</Button>}
                title={t("Create new recovery codes")}
                description={t("The new codes replace those you have, which stop working.")}
                action={t("Create codes")}
                change={renew}
                onSuccess={({ recoveryCodes }) => setCodes(recoveryCodes)}
              />
              <ConfirmPasswordDialog
                trigger={<Button variant="destructive">{t("Turn off")}</Button>}
                title={t("Turn off two-factor authentication")}
                description={t("Then your password alone signs you in. Your authenticator app and recovery codes stop working.")}
                action={t("Turn off")}
                destructive
                change={disable}
                onSuccess={() => toast.success(t("Turned off two-factor authentication"))}
              />
            </>
          }
        >
          {t("Signing in asks for a code from the app.")}{" "}
          <span className={data.recoveryCodes <= 3 ? "font-medium text-warning" : undefined}>
            {t("{{count}} recovery codes left.", { count: data.recoveryCodes, defaultValue_one: "{{count}} recovery code left." })}
          </span>
        </AccountRow>
      ) : (
        <AccountRow
          icon={ShieldIcon}
          tone="neutral"
          title={t("Authenticator app")}
          status={{ tone: "neutral", label: msg("Off") }}
          actions={
            <MfaSetupDialog
              username={username}
              enable={enable}
              onEnabled={(codes) => {
                setCodes(codes)
                toast.success(t("Turned on two-factor authentication"), { description: t("You were signed out everywhere else.") })
              }}
            />
          }
        >
          {t("Your password alone signs you in.")}
        </AccountRow>
      )}
      <RecoveryCodesDialog codes={codes} onClose={() => setCodes(undefined)} />
    </>
  )
}

/** A way of signing in, with what it is and the actions that change it. */
function AccountRow({
  icon,
  tone,
  title,
  status,
  actions,
  children,
}: {
  icon: Icon
  tone: Tone
  title: string
  status?: { tone: Tone; label: string }
  actions: ReactNode
  children: ReactNode
}) {
  return (
    <div className="flex flex-wrap items-center gap-4 rounded-xl border p-4">
      <IconTile icon={icon} tone={tone} />
      <div className="min-w-48 flex-1 space-y-0.5">
        <p className="flex items-center gap-2 font-medium">
          {title}
          {status && <StatusBadge status={status} />}
        </p>
        <p className="text-sm text-muted-foreground">{children}</p>
      </div>
      <div className="flex flex-wrap gap-2">{actions}</div>
    </div>
  )
}
