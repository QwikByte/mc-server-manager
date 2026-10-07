import { DeviceMobileIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { useState } from "react"
import { Button } from "@/components/ui/button"
import { meQuery, useEnableMfa, useLogout } from "./api"
import { AuthLayout } from "./auth-layout"
import { MfaSetupDialog, RecoveryCodesDialog } from "./mfa-dialogs"

const route = getRouteApi("/two-factor")

/**
 * Where users whom the settings require to use two-factor authentication set it up before anything else. Once
 * they saved their recovery codes, they go on to where they were going.
 */
export function TwoFactorPage() {
  const { redirect } = route.useSearch()
  const navigate = useNavigate()
  const { data: user } = useQuery(meQuery)
  const enable = useEnableMfa()
  const logout = useLogout()
  const [codes, setCodes] = useState<string[]>()

  return (
    <AuthLayout
      title={t("Set up two-factor authentication")}
      description={t("It is required for your account. Until you set it up, the panel offers nothing else.")}
    >
      <div className="grid gap-4">
        <p className="text-sm text-muted-foreground">
          {t("After your password, signing in will also ask for a code from an authenticator app on your phone.")}
        </p>
        {user && (
          <MfaSetupDialog
            username={user.username}
            enable={enable}
            onEnabled={setCodes}
            trigger={
              <Button size="lg" className="w-full">
                <DeviceMobileIcon /> {t("Set up")}
              </Button>
            }
          />
        )}
        <Button
          variant="ghost"
          className="-mt-2 w-full"
          disabled={logout.isPending}
          onClick={() => logout.mutate(undefined, { onSettled: () => navigate({ to: "/login", search: {} }) })}
        >
          {t("Sign out")}
        </Button>
      </div>
      <RecoveryCodesDialog codes={codes} onClose={() => navigate({ to: redirect ?? "/" })} />
    </AuthLayout>
  )
}
