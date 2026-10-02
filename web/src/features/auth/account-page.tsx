import { type Icon, KeyIcon, ShieldCheckIcon, ShieldIcon, UserCircleIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { type ReactNode, useState } from "react"
import { toast } from "sonner"
import { ErrorCallout } from "@/components/callout"
import { FormSection } from "@/components/form-section"
import { IconTile } from "@/components/icon-tile"
import { PageHeader } from "@/components/page-header"
import { StatusBadge } from "@/components/status"
import type { Tone } from "@/components/tone"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { meQuery, mfaQuery, useDisableMfa, useEnableMfa, useNewRecoveryCodes } from "./api"
import { ConfirmPasswordDialog, MfaSetupDialog, RecoveryCodesDialog } from "./mfa-dialogs"
import { PasswordDialog } from "./password-dialog"

/** The signed-in user's own account: how they sign in. */
export function AccountPage() {
  const { data: user } = useQuery(meQuery)
  if (!user) return null
  return (
    <>
      <PageHeader icon={UserCircleIcon} tone="violet" title="Your account" description={`Signed in as ${user.username}.`} />
      <div className="surface rounded-2xl px-5 sm:px-8 [&>section:last-child]:border-b-0">
        <FormSection title="Password" description="Changing it signs you out on your other devices.">
          <AccountRow icon={KeyIcon} tone="info" title="Password" actions={<PasswordDialog username={user.username} />}>
            At least 12 characters that you don't use anywhere else.
          </AccountRow>
        </FormSection>
        <FormSection
          title="Two-factor authentication"
          description="A code of an authenticator app on your phone keeps your account safe even if someone learns your password."
        >
          <MfaSettings username={user.username} />
        </FormSection>
      </div>
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
          title="Authenticator app"
          status={{ tone: "success", label: "On" }}
          actions={
            <>
              <ConfirmPasswordDialog
                trigger={<Button variant="outline">New recovery codes</Button>}
                title="Create new recovery codes"
                description="The new codes replace those you have, which stop working."
                action="Create codes"
                change={renew}
                onSuccess={({ recoveryCodes }) => setCodes(recoveryCodes)}
              />
              <ConfirmPasswordDialog
                trigger={<Button variant="destructive">Turn off</Button>}
                title="Turn off two-factor authentication"
                description="Then your password alone signs you in. Your authenticator app and recovery codes stop working."
                action="Turn off"
                destructive
                change={disable}
                onSuccess={() => toast.success("Turned off two-factor authentication")}
              />
            </>
          }
        >
          Signing in asks for a code of the app.{" "}
          <span className={data.recoveryCodes <= 3 ? "font-medium text-warning" : undefined}>
            {data.recoveryCodes === 1 ? "1 recovery code" : `${data.recoveryCodes} recovery codes`} left.
          </span>
        </AccountRow>
      ) : (
        <AccountRow
          icon={ShieldIcon}
          tone="neutral"
          title="Authenticator app"
          status={{ tone: "neutral", label: "Off" }}
          actions={
            <MfaSetupDialog
              username={username}
              enable={enable}
              onEnabled={(codes) => {
                setCodes(codes)
                toast.success("Turned on two-factor authentication", { description: "You were signed out everywhere else." })
              }}
            />
          }
        >
          Your password alone signs you in.
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
