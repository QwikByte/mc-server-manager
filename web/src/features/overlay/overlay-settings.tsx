import { GearIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { Section } from "@/components/section"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { useAccess } from "@/features/access/use-access"
import { type OverlaySettings, overlayQuery, useUpdateOverlaySettings } from "./api"

/** The settings of the private network of the nodes and how many nodes are part of it. */
export function OverlaySettingsSection() {
  const manage = useAccess().can("overlay.manage")
  const { data } = useQuery(overlayQuery)
  const [editing, setEditing] = useState(false)
  if (!data) return null
  const facts = [
    [t("Range"), data.subnet],
    [t("UDP port"), String(data.port)],
    [t("MTU"), String(data.mtu)],
    [t("Members"), String(data.members.length)],
  ]
  return (
    <Section
      title={t("Private network")}
      description={t("Nodes join it from their own page once their administrator allows it. Open its UDP port between them.")}
      actions={
        manage && (
          <Button variant="outline" onClick={() => setEditing(true)}>
            <GearIcon />
            {t("Settings")}
          </Button>
        )
      }
    >
      <dl className="surface grid grid-cols-2 gap-x-6 gap-y-4 rounded-xl px-5 py-4 md:grid-cols-4">
        {facts.map(([term, value]) => (
          <div key={term}>
            <dt className="text-xs text-muted-foreground">{term}</dt>
            <dd className="mt-0.5 font-mono text-sm">{value}</dd>
          </div>
        ))}
      </dl>
      {editing && <SettingsDialog settings={data} members={data.members.length} onClose={() => setEditing(false)} />}
    </Section>
  )
}

function SettingsDialog({ settings, members, onClose }: { settings: OverlaySettings; members: number; onClose: () => void }) {
  const [draft, setDraft] = useState(settings)
  const update = useUpdateOverlaySettings()

  function submit(event: FormEvent) {
    event.preventDefault()
    update.mutate(draft, { onSuccess: onClose })
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{t("Private network")}</DialogTitle>
            <DialogDescription>{t("The members get a new port or MTU right away.")}</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="overlay-subnet">{t("Range")}</FieldLabel>
              <Input
                id="overlay-subnet"
                className="font-mono"
                value={draft.subnet}
                disabled={members > 0}
                onChange={(e) => setDraft({ ...draft, subnet: e.target.value })}
              />
              <FieldDescription>
                {members > 0
                  ? t("Only changes while no node is part of it.")
                  : t("A private IPv4 range that overlaps no network of the nodes, e.g. 10.213.0.0/24.")}
              </FieldDescription>
            </Field>
            <div className="grid grid-cols-2 gap-4">
              <Field>
                <FieldLabel htmlFor="overlay-port">{t("UDP port")}</FieldLabel>
                <Input id="overlay-port" type="number" min={1024} max={65535} value={draft.port} onChange={(e) => setDraft({ ...draft, port: Number(e.target.value) })} />
              </Field>
              <Field>
                <FieldLabel htmlFor="overlay-mtu">{t("MTU")}</FieldLabel>
                <Input id="overlay-mtu" type="number" min={1280} max={1500} value={draft.mtu} onChange={(e) => setDraft({ ...draft, mtu: Number(e.target.value) })} />
              </Field>
            </div>
            {update.error && <FieldError>{update.error.message}</FieldError>}
          </FieldGroup>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" disabled={update.isPending}>
              {t("Save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
