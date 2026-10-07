import { ArrowUpIcon, CaretLeftIcon, DownloadSimpleIcon } from "@phosphor-icons/react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { t } from "i18next"
import { lazy, Suspense } from "react"
import { toast } from "sonner"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { formatBytes } from "@/lib/format"
import { contentUrl, type ServerFiles } from "./api"
import { type Log, maxUnpacked, readEarlier, readEnd, unpack } from "./log-reader"

// CodeMirror is only loaded when a log is opened.
const LogEditor = lazy(() => import("./log-editor").then((m) => ({ default: m.LogEditor })))

/**
 * Shows a log of a server, read only: the end of a large log, with earlier parts on request, and archived
 * logs (.log.gz) unpacked in the browser.
 */
export function LogViewer({ files, path, onClose }: { files: ServerFiles; path: string; onClose: () => void }) {
  const name = path.split("/").pop() ?? path
  const archive = /\.gz$/i.test(name)
  const queryClient = useQueryClient()
  const key = ["log", files.nodeId, files.serverId, path]
  const {
    data: log,
    error,
    isPending,
  } = useQuery({
    queryKey: key,
    queryFn: () => (archive ? unpack(files, path) : readEnd(files, path)),
    staleTime: Infinity,
    gcTime: 0,
    retry: false,
  })
  // Lines read before are added in front of the text.
  const earlier = useMutation({
    mutationFn: (shown: Log) => readEarlier(files, path, shown),
    onSuccess: (next) => queryClient.setQueryData(key, next),
    onError: (e) => toast.error(e.message),
  })

  return (
    <section aria-labelledby="log-heading">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <Button variant="ghost" size="sm" onClick={onClose}>
            <CaretLeftIcon />
            {t("Back")}
          </Button>
          <h2 id="log-heading" className="truncate font-mono text-sm">
            {path}
          </h2>
          <Pill tone="neutral">{t("Read only")}</Pill>
        </div>
        <Button variant="outline" asChild>
          <a href={contentUrl(files, path)} download>
            <DownloadSimpleIcon />
            {t("Download")}
          </a>
        </Button>
      </div>
      {log && (log.start > 0 || log.truncated) && (
        <div className="mb-3 flex flex-wrap items-center gap-x-3 gap-y-2 text-sm text-muted-foreground">
          <p>
            {log.truncated
              ? t("Shows the first {{size}} of the unpacked log. Download it to read all of it.", { size: formatBytes(maxUnpacked) })
              : t("Shows the last {{size}} of {{total}}.", { size: formatBytes(log.size - log.start), total: formatBytes(log.size) })}
          </p>
          {log.start > 0 && (
            <Button variant="outline" size="sm" disabled={earlier.isPending} onClick={() => earlier.mutate(log)}>
              <ArrowUpIcon />
              {earlier.isPending ? t("Loading…") : t("Load earlier")}
            </Button>
          )}
        </div>
      )}
      {isPending ? (
        <Skeleton className="h-[65vh] min-h-80 rounded-xl" />
      ) : error ? (
        <p role="alert" className="rounded-xl border border-dashed px-4 py-14 text-center text-sm text-muted-foreground">
          {error.message}
        </p>
      ) : (
        <Suspense fallback={<Skeleton className="h-[65vh] min-h-80 rounded-xl" />}>
          <LogEditor text={log.text} filename={name} earlierEnd={log.added} />
        </Suspense>
      )}
    </section>
  )
}
