import { ClockCounterClockwiseIcon, FilesIcon, GearSixIcon, KeyIcon, PaperPlaneTiltIcon, TrashIcon, UsersThreeIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useBlocker, useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { useState } from "react"
import { toast } from "sonner"
import { BackLink } from "@/components/back-link"
import { ErrorCallout } from "@/components/callout"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { PageHeader } from "@/components/page-header"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { msg } from "@/lib/i18n"
import { type FileSet, fileSetQuery, type SetInput, useDeleteFileSet, useSaveFileSet } from "./api"
import { ApplyDialog } from "./apply-dialog"
import { FilesTab } from "./files-tab"
import { HistoryTab } from "./history-tab"
import { SecretsTab } from "./secrets-tab"
import { ServersTab } from "./servers-tab"
import { SettingsTab } from "./settings-tab"

const route = getRouteApi("/_app/filesets/$fileSetId")

const draftOf = ({ name, description, files, targets, version }: FileSet): SetInput => ({ name, description, files, targets, version })

const tabs = [
  { id: "files", label: msg("Files"), icon: FilesIcon },
  { id: "settings", label: msg("Targets and settings"), icon: GearSixIcon },
  { id: "secrets", label: msg("Secrets"), icon: KeyIcon },
  { id: "servers", label: msg("Servers"), icon: UsersThreeIcon },
  { id: "history", label: msg("History"), icon: ClockCounterClockwiseIcon },
] as const

type Tab = (typeof tabs)[number]["id"]

export function FileSetPage() {
  const { fileSetId } = route.useParams()
  const { data: set, isPending, error } = useQuery(fileSetQuery(fileSetId))
  return (
    <>
      <BackLink to="/filesets">{t("File sets")}</BackLink>
      {isPending ? <Skeleton className="h-96 rounded-xl" /> : error ? <ErrorCallout error={error} /> : <SetEditor key={set.id} set={set} />}
    </>
  )
}

/** A set with its files and settings as they are edited, which saving stores as a new version. */
function SetEditor({ set }: { set: FileSet }) {
  const editable = useAccess().can("filesets.manage")
  const [draft, setDraft] = useState(() => draftOf(set))
  const [dirty, setDirty] = useState(false)
  // Saving or discarding loads the stored set into the editors again.
  const [generation, setGeneration] = useState(0)
  const [tab, setTab] = useState<Tab>("files")
  const [selected, setSelected] = useState<string>()
  const [applying, setApplying] = useState(false)
  const save = useSaveFileSet(set.id)
  const remove = useDeleteFileSet()
  const navigate = useNavigate()
  const blocker = useBlocker({ shouldBlockFn: () => dirty && !save.isPending, enableBeforeUnload: () => dirty, withResolver: true })

  // Without changes of its own, the editor follows what others save.
  const [shown, setShown] = useState(set)
  if (set !== shown && !dirty) {
    setShown(set)
    setDraft(draftOf(set))
    setGeneration((g) => g + 1)
  }

  const change = (next: Partial<SetInput>) => {
    setDraft((d) => ({ ...d, ...next }))
    setDirty(true)
  }
  const reset = (to: FileSet) => {
    setDraft(draftOf(to))
    setDirty(false)
    setGeneration((g) => g + 1)
  }

  return (
    <>
      <PageHeader
        icon={FilesIcon}
        tone="info"
        title={set.name}
        description={set.description || undefined}
        badge={<Pill tone="neutral">{t("Version {{version}}", { version: set.version })}</Pill>}
        actions={
          editable && (
            <>
              <Button
                disabled={dirty}
                title={dirty ? t("Save your changes first.") : t("Preview what changes on each server, then apply.")}
                onClick={() => setApplying(true)}
              >
                <PaperPlaneTiltIcon />
                {t("Apply…")}
              </Button>
              <ConfirmDialog
                trigger={
                  <Button variant="outline" className="text-destructive hover:bg-destructive/10 hover:text-destructive">
                    <TrashIcon />
                    {t("Delete")}
                  </Button>
                }
                title={t("Delete {{name}}?", { name: set.name })}
                description={t("The servers lose the files of the set that hold secrets. Its other files stay on the servers as they are.")}
                action={t("Delete file set")}
                destructive
                onConfirm={() =>
                  remove.mutate(set.id, {
                    onSuccess: () => {
                      toast.success(t("Deleted {{name}}", { name: set.name }))
                      void navigate({ to: "/filesets", ignoreBlocker: true })
                    },
                    onError: (e) => toast.error(e.message),
                  })
                }
              />
            </>
          )
        }
      />
      <div role="tablist" aria-label={t("Parts of the file set")} className="mb-8 flex max-w-full gap-1 overflow-x-auto rounded-xl bg-muted/80 p-1 sm:w-fit">
        {tabs.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            type="button"
            role="tab"
            aria-selected={tab === id}
            onClick={() => setTab(id)}
            className="flex items-center gap-2 rounded-lg px-3.5 py-1.5 text-sm font-medium whitespace-nowrap text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring aria-selected:bg-card aria-selected:text-foreground aria-selected:shadow-sm"
          >
            <Icon className="size-4" />
            {t(label)}
          </button>
        ))}
      </div>
      {tab === "files" && (
        <FilesTab
          key={generation}
          files={draft.files}
          editable={editable}
          selected={selected}
          onSelect={setSelected}
          onChange={(files) => change({ files })}
        />
      )}
      {tab === "settings" && (
        <fieldset disabled={!editable} className="contents">
          <SettingsTab draft={draft} onChange={change} />
        </fieldset>
      )}
      {tab === "secrets" && <SecretsTab set={set} files={draft.files} editable={editable} />}
      {tab === "servers" && <ServersTab set={set} />}
      {tab === "history" && (
        <HistoryTab
          set={set}
          dirty={dirty}
          editable={editable}
          onRestore={(files) => {
            change({ files })
            setGeneration((g) => g + 1)
            setTab("files")
          }}
        />
      )}

      {dirty && (
        <div className="sticky bottom-4 z-10 mt-10 flex flex-wrap items-center justify-between gap-3 rounded-2xl bg-popover/90 px-4 py-3 shadow-2xl ring-1 ring-foreground/10 backdrop-blur-xl">
          <div className="min-w-0 space-y-0.5">
            <p className="flex items-center gap-2.5 text-sm font-medium">
              <span aria-hidden className="size-2 rounded-full bg-warning" />
              {t("Unsaved changes")}
            </p>
            <p className={save.error ? "text-xs text-destructive" : "text-xs text-muted-foreground"}>
              {save.error?.message ?? t("Saving creates a new version and changes no server until the set is applied.")}
            </p>
          </div>
          <div className="flex gap-2">
            <Button variant="ghost" disabled={save.isPending} onClick={() => reset(set)}>
              {t("Discard")}
            </Button>
            <Button
              disabled={save.isPending}
              onClick={() =>
                save.mutate(draft, {
                  onSuccess: (saved) => {
                    reset(saved)
                    toast.success(t("Saved {{name}}", { name: saved.name }))
                  },
                })
              }
            >
              {t("Save")}
            </Button>
          </div>
        </div>
      )}
      {applying && <ApplyDialog set={set} onClose={() => setApplying(false)} />}
      <ConfirmDialog
        open={blocker.status === "blocked"}
        onOpenChange={(open) => !open && blocker.reset?.()}
        title={t("Discard your changes?")}
        description={t("Your changes to the file set haven't been saved.")}
        action={t("Discard changes")}
        destructive
        onConfirm={() => blocker.proceed?.()}
      />
    </>
  )
}
