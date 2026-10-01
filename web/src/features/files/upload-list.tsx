import { CheckCircleIcon, WarningCircleIcon, XIcon } from "@phosphor-icons/react"
import { Button } from "@/components/ui/button"
import { Progress } from "@/components/ui/progress"
import type { Upload } from "./use-uploads"

export function UploadList({ uploads, onCancel }: { uploads: Upload[]; onCancel: (id: number) => void }) {
  if (uploads.length === 0) return null
  return (
    <ul className="mb-4 divide-y border bg-card" aria-label="Uploads">
      {uploads.map((u) => (
        <li key={u.id} className="flex items-center gap-3 px-3 py-2 text-sm">
          {u.state === "done" ? (
            <CheckCircleIcon className="size-4 shrink-0 text-primary" />
          ) : u.state === "failed" ? (
            <WarningCircleIcon className="size-4 shrink-0 text-destructive" />
          ) : null}
          <span className="min-w-0 flex-1 truncate font-mono text-xs">{u.name}</span>
          {u.state === "failed" ? (
            <span className="text-xs text-destructive">{u.error}</span>
          ) : u.state === "done" ? (
            <span className="text-xs text-muted-foreground">Uploaded</span>
          ) : (
            <>
              <Progress value={u.progress * 100} className="w-32 sm:w-48" aria-label={`Upload of ${u.name}`} />
              <span className="w-10 text-right text-xs text-muted-foreground tabular-nums">
                {u.state === "queued" ? "–" : `${Math.floor(u.progress * 100)}%`}
              </span>
            </>
          )}
          <Button
            size="icon-xs"
            variant="ghost"
            aria-label={u.state === "uploading" || u.state === "queued" ? `Cancel upload of ${u.name}` : `Dismiss ${u.name}`}
            onClick={() => onCancel(u.id)}
          >
            <XIcon />
          </Button>
        </li>
      ))}
    </ul>
  )
}
