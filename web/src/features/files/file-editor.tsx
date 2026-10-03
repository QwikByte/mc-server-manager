import { CaretLeftIcon, DownloadSimpleIcon, FloppyDiskIcon } from "@phosphor-icons/react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useBlocker } from "@tanstack/react-router"
import { t } from "i18next"
import { lazy, Suspense, useEffect, useRef, useState } from "react"
import { Trans } from "react-i18next"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { contentUrl, readText, type ServerFiles, upload } from "./api"
import type { EditorHandle } from "./code-editor"

// CodeMirror is only loaded when a file is opened.
const CodeEditor = lazy(() => import("./code-editor").then((m) => ({ default: m.CodeEditor })))

/** Edits a text file of a server. Leaving with unsaved changes asks first. */
export function FileEditor({ files, path, onClose }: { files: ServerFiles; path: string; onClose: () => void }) {
  const writable = useAccess().can("files.write", files.nodeId, files.serverId)
  const name = path.split("/").pop() ?? path
  const queryClient = useQueryClient()
  const editor = useRef<EditorHandle>(null)
  const [dirty, setDirty] = useState(false)
  const {
    data: text,
    error,
    isPending,
  } = useQuery({
    queryKey: ["file", files.nodeId, files.serverId, path],
    queryFn: () => readText(files, path),
    staleTime: Infinity,
    gcTime: 0,
    retry: false,
  })
  const save = useMutation({
    mutationFn: () => upload(files, path, editor.current?.value() ?? "", { overwrite: true }),
    onSuccess: () => {
      setDirty(false)
      toast.success(t("Saved {{name}}", { name }))
      void queryClient.invalidateQueries({ queryKey: ["files", files.nodeId, files.serverId] })
    },
    onError: (e) => toast.error(e.message),
  })
  const blocker = useBlocker({ shouldBlockFn: () => dirty, enableBeforeUnload: () => dirty, withResolver: true })
  const { mutate } = save

  // Ctrl+S saves wherever the focus is, e.g. after a dialog closed.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
        e.preventDefault()
        if (dirty) mutate()
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [dirty, mutate])

  return (
    <section aria-labelledby="editor-heading">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <Button variant="ghost" size="sm" onClick={onClose}>
            <CaretLeftIcon />
            {t("Back")}
          </Button>
          <h2 id="editor-heading" className="truncate font-mono text-sm">
            {path}
          </h2>
          {dirty && <Pill tone="warning">{t("Unsaved")}</Pill>}
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <a href={contentUrl(files, path)} download>
              <DownloadSimpleIcon />
              {t("Download")}
            </a>
          </Button>
          {writable && (
            <Button disabled={!dirty || save.isPending} onClick={() => save.mutate()}>
              <FloppyDiskIcon />
              {save.isPending ? t("Saving…") : t("Save")}
            </Button>
          )}
        </div>
      </div>
      {isPending ? (
        <Skeleton className="h-[65vh] min-h-80 rounded-xl" />
      ) : error ? (
        <p role="alert" className="rounded-xl border border-dashed px-4 py-14 text-center text-sm text-muted-foreground">
          {error.message}
        </p>
      ) : (
        <Suspense fallback={<Skeleton className="h-[65vh] min-h-80 rounded-xl" />}>
          <CodeEditor ref={editor} value={text} filename={name} readOnly={!writable} onChange={() => setDirty(true)} />
        </Suspense>
      )}
      <p className="mt-3 text-xs text-muted-foreground" hidden={!writable}>
        <Trans
          i18nKey="<key>Ctrl+S</key> saves. Servers read most settings when they start, so restart the server to apply your changes."
          components={{ key: <kbd className="rounded-md border bg-muted px-1.5 py-0.5 font-mono text-[0.6875rem]" /> }}
        />
      </p>
      <ConfirmDialog
        open={blocker.status === "blocked"}
        onOpenChange={(open) => !open && blocker.reset?.()}
        title={t("Discard your changes?")}
        description={t("The changes to {{name}} haven't been saved.", { name })}
        action={t("Discard changes")}
        destructive
        onConfirm={() => blocker.proceed?.()}
      />
    </section>
  )
}
