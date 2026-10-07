import { CloudSlashIcon } from "@phosphor-icons/react"
import { onlineManager } from "@tanstack/react-query"
import { t } from "i18next"
import { useEffect, useSyncExternalStore } from "react"
import { Callout } from "@/components/callout"
import { masterReachable, request } from "@/lib/api"

/** Tells that the browser is offline or the master doesn't answer, e.g. while it restarts, until it does again. */
export function ConnectionBanner() {
  const online = useSyncExternalStore(onlineManager.subscribe.bind(onlineManager), () => onlineManager.isOnline())
  const reachable = useSyncExternalStore(masterReachable.subscribe, masterReachable.get)
  // Asks the master now and then while it doesn't answer, also on pages that load nothing on their own.
  useEffect(() => {
    if (!online || reachable) return
    const timer = setInterval(() => void request("/auth/me").catch(() => {}), 5_000)
    return () => clearInterval(timer)
  }, [online, reachable])

  if (online && reachable) return null
  return (
    <Callout
      tone="warning"
      icon={CloudSlashIcon}
      role="status"
      className="mb-6"
      title={online ? t("The master doesn't answer") : t("You're offline")}
    >
      {online
        ? t("It may be restarting. The panel carries on once it answers again.")
        : t("The panel carries on once the connection is back.")}
    </Callout>
  )
}
