import { FileArchiveIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { Segmented } from "@/components/segmented"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldContent, FieldDescription, FieldError, FieldGroup, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { join } from "./api"

/** The name of the folder an archive suggests, e.g. "world" for world.zip. */
const stem = (name: string) => name.replace(/\.(zip|tar\.gz|tgz)$/i, "")

/**
 * Asks where to extract an archive of a folder: into the folder, or into a new folder in it named like the archive,
 * and whether existing files are replaced.
 */
export function ExtractDialog({
  dir,
  name,
  onClose,
  onExtract,
}: {
  dir: string
  name: string
  onClose: () => void
  onExtract: (destination: string, overwrite: boolean) => void
}) {
  const [where, setWhere] = useState<"here" | "folder">("here")
  const [folder, setFolder] = useState(stem(name))
  const [overwrite, setOverwrite] = useState(false)
  const trimmed = folder.trim().replace(/^\/+|\/+$/g, "")
  const invalid = where === "folder" && (!trimmed || trimmed.includes("\\") || trimmed.split("/").some((part) => part === "." || part === ".."))

  function submit(event: FormEvent) {
    event.preventDefault()
    if (invalid) return
    onClose()
    onExtract(where === "here" ? dir : join(dir, trimmed), overwrite)
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{t("Extract {{name}}", { name })}</DialogTitle>
            <DialogDescription>
              {t(
                "ZIP and .tar.gz archives. Archives with links, paths outside the folder, files with secrets or files that Noryx manages itself, such as server.properties, are refused before anything is written.",
              )}
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <FieldSet>
              <FieldLegend variant="label">{t("Extract into")}</FieldLegend>
              <Segmented
                label={t("Extract into")}
                className="w-fit"
                value={where}
                onChange={setWhere}
                options={[
                  { value: "here", label: dir ? t("This folder") : t("The server folder") },
                  { value: "folder", label: t("A new folder") },
                ]}
              />
              {where === "folder" && (
                <Field data-invalid={invalid}>
                  <Input
                    aria-label={t("Name of the new folder")}
                    className="font-mono"
                    autoFocus
                    value={folder}
                    onChange={(e) => setFolder(e.target.value)}
                  />
                  {invalid && <FieldError>{t("Enter the name of a folder, e.g. world.")}</FieldError>}
                </Field>
              )}
            </FieldSet>
            <Field orientation="horizontal">
              <Switch id="extract-overwrite" checked={overwrite} onCheckedChange={setOverwrite} />
              <FieldContent>
                <FieldLabel htmlFor="extract-overwrite">{t("Replace existing files")}</FieldLabel>
                <FieldDescription>{t("Otherwise an archive with files that exist already is refused.")}</FieldDescription>
              </FieldContent>
            </Field>
          </FieldGroup>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={invalid}>
              <FileArchiveIcon />
              {t("Extract")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
