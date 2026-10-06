import { t } from "i18next"
import { Trans } from "react-i18next"
import { CopyField } from "@/components/copy-field"
import { formatDateTime } from "@/lib/format"
import { type SetupLink, setupUrl } from "./api"

/** A setup link to pass on to its user, e.g. in a direct message. */
export function SetupLinkView({ username, link }: { username: string; link: SetupLink }) {
  return (
    <div className="space-y-3 text-sm">
      <p>
        <Trans
          i18nKey="Send this link to <user/>. It can be used once to set a password and works until <time/>."
          components={{ user: <span className="font-medium">{username}</span>, time: <>{formatDateTime(link.expiresAt)}</> }}
        />
      </p>
      <CopyField label={t("Setup link")} value={setupUrl(link)} />
      <p className="text-xs text-muted-foreground">
        {t("Anyone with the link can sign in as {{name}}, so share it only with them.", { name: username })}
      </p>
    </div>
  )
}
