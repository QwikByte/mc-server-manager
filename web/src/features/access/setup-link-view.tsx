import { CopyField } from "@/components/copy-field"
import { formatDateTime } from "@/lib/format"
import { type SetupLink, setupUrl } from "./api"

/** A setup link to pass on to its user, e.g. in a direct message. */
export function SetupLinkView({ username, link }: { username: string; link: SetupLink }) {
  return (
    <div className="space-y-3 text-sm">
      <p>
        Send this link to <span className="font-medium">{username}</span>. It sets the password once and works until{" "}
        {formatDateTime(link.expiresAt)}.
      </p>
      <CopyField label="Setup link" value={setupUrl(link)} />
      <p className="text-xs text-muted-foreground">Anyone with the link can sign in as {username}, so share it only with them.</p>
    </div>
  )
}
