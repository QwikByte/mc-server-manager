import { t } from "i18next"
import { Progress } from "@/components/ui/progress"

/** How far an upload of an archive got; once it is sent, the agent checks and unpacks it. */
export function UploadProgress({ progress, done }: { progress: number; done: string }) {
  const sent = progress >= 1
  return (
    <div className="grid gap-2" aria-live="polite">
      <Progress value={sent ? 100 : progress * 100} aria-label={t("Upload progress")} className={sent ? "animate-pulse" : undefined} />
      <p className="text-sm text-muted-foreground tabular-nums">
        {sent ? done : t("Uploading… {{percent}}%", { percent: Math.floor(progress * 100) })}
      </p>
    </div>
  )
}
