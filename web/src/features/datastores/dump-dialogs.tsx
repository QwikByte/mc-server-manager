import { ArchiveIcon, ArrowCounterClockwiseIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { type FormEvent, useId, useState } from "react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Field, FieldContent, FieldDescription, FieldError, FieldGroup, FieldLabel, FieldLegend, FieldSet, FieldTitle } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { guard, useOperation } from "@/features/operations/use-operation"
import { formatBytes, formatDateTime } from "@/lib/format"
import { type Datastore, type Dump, useDatastore } from "./api"

/**
 * Backs up databases of a datastore by hand, all at first. Once the master dumps them, the dialog closes and a
 * notification follows the backup; a request that fails right away shows its error in the dialog.
 */
export function BackUpDialog({ datastore: ds }: { datastore: Datastore }) {
  // Databases the datastore lacks can't be dumped.
  const names = ds.databases.map((db) => db.name).filter((name) => !ds.missing.includes(name))
  const [open, setOpen] = useState(false)
  const [label, setLabel] = useState("")
  const [databases, setDatabases] = useState<string[]>([])
  const chosen = databases.filter((name) => names.includes(name))
  const { dump } = useDatastore(ds.id)
  const operation = useOperation()

  function onOpenChange(next: boolean) {
    setOpen(next)
    if (next) {
      dump.reset()
      setLabel("")
      setDatabases(names)
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    const title = t("Backing up {{name}}…", { name: ds.name })
    operation.run(
      (onStart) =>
        dump.mutateAsync({
          label: label.trim(),
          databases: chosen,
          onStart: (op) => {
            onStart(op)
            operation.background(title, op.id)
            setOpen(false)
          },
        }),
      {
        title,
        done: (d) => ({ message: t("Backed up {{name}} ({{size}})", { name: ds.name, size: formatBytes(d.size) }) }),
        then: () => setOpen(false),
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline" disabled={dump.isPending || ds.state !== "running" || names.length === 0}>
          <ArchiveIcon />
          {dump.isPending ? t("Backing up…") : t("Back up now")}
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg" {...guard(dump.isPending)}>
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{t("Back up {{name}}", { name: ds.name })}</DialogTitle>
            <DialogDescription>
              {t("The databases are dumped while the datastore keeps running. The backup is kept on {{node}}.", { node: ds.nodeName })}
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="dump-label">{t("Label")}</FieldLabel>
              <Input
                id="dump-label"
                maxLength={64}
                placeholder={t("Before the update")}
                value={label}
                onChange={(e) => setLabel(e.target.value)}
              />
            </Field>
            <DatabasesField legend={t("Databases to back up")} names={names} value={chosen} onChange={setDatabases} />
          </FieldGroup>
          {dump.error && <FieldError>{dump.error.message}</FieldError>}
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline" disabled={dump.isPending}>
                {t("Cancel")}
              </Button>
            </DialogClose>
            <Button type="submit" disabled={chosen.length === 0 || dump.isPending}>
              {dump.isPending ? t("Backing up…") : t("Back up")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

/**
 * Restores the chosen databases of a dump; one that was dropped since has to be added again first. Once the master
 * restores them, the dialog closes and a notification follows.
 */
export function RestoreDialog({ datastore: ds, dump: d }: { datastore: Datastore; dump: Dump }) {
  const known = (name: string) => ds.databases.some((db) => db.name === name)
  const [open, setOpen] = useState(false)
  const [databases, setDatabases] = useState<string[]>([])
  const chosen = databases.filter(known)
  const { restore } = useDatastore(ds.id)
  const operation = useOperation()
  const created = formatDateTime(d.createdAt)

  function onOpenChange(next: boolean) {
    setOpen(next)
    if (next) {
      restore.reset()
      setDatabases(d.databases.filter(known))
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    const title = t("Restoring {{name}}…", { name: ds.name })
    operation.run(
      (onStart) =>
        restore.mutateAsync({
          dump: d.id,
          databases: chosen,
          onStart: (op) => {
            onStart(op)
            operation.background(title, op.id)
            setOpen(false)
          },
        }),
      {
        title,
        done: () => ({ message: t("Restored the backup of {{time}}", { time: created }) }),
        then: () => setOpen(false),
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline" disabled={restore.isPending}>
          <ArrowCounterClockwiseIcon />
          <span className="max-sm:sr-only">{t("Restore")}</span>
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg" {...guard(restore.isPending)}>
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{t("Restore the backup of {{time}}?", { time: created })}</DialogTitle>
            <DialogDescription>
              {t(
                "The chosen databases get their backed up state; what was added since is lost. The plugins that use them lose their connection meanwhile, so stop their servers first.",
              )}
            </DialogDescription>
          </DialogHeader>
          <DatabasesField
            legend={t("Databases to restore")}
            names={d.databases}
            value={chosen}
            onChange={setDatabases}
            unavailable={(name) => (known(name) ? undefined : t("Add the database again to restore it."))}
          />
          {restore.error && <FieldError>{restore.error.message}</FieldError>}
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline" disabled={restore.isPending}>
                {t("Cancel")}
              </Button>
            </DialogClose>
            <Button type="submit" variant="destructive" disabled={chosen.length === 0 || restore.isPending}>
              {restore.isPending ? t("Restoring…") : t("Restore")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

/** Chooses databases; one that can't be chosen tells why. */
function DatabasesField({
  legend,
  names,
  value,
  onChange,
  unavailable,
}: {
  legend: string
  names: string[]
  value: string[]
  onChange: (names: string[]) => void
  unavailable?: (name: string) => string | undefined
}) {
  const id = useId()
  return (
    <FieldSet>
      <FieldLegend variant="label">{legend}</FieldLegend>
      <div className="grid gap-2 sm:grid-cols-2">
        {names.map((name) => {
          const why = unavailable?.(name)
          return (
            <FieldLabel key={name} htmlFor={`${id}-${name}`}>
              <Field orientation="horizontal" data-disabled={!!why}>
                <Checkbox
                  id={`${id}-${name}`}
                  checked={value.includes(name)}
                  disabled={!!why}
                  onCheckedChange={(on) => onChange(on === true ? [...value, name] : value.filter((other) => other !== name))}
                />
                <FieldContent>
                  <FieldTitle>
                    <span className="font-mono">{name}</span>
                  </FieldTitle>
                  {why && <FieldDescription>{why}</FieldDescription>}
                </FieldContent>
              </Field>
            </FieldLabel>
          )
        })}
      </div>
    </FieldSet>
  )
}
