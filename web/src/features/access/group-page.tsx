import { CrownIcon, GlobeIcon, TargetIcon, TrashIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useBlocker, useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { toast } from "sonner"
import { BackLink } from "@/components/back-link"
import { Callout, ErrorCallout } from "@/components/callout"
import { Chip } from "@/components/chip"
import { ConfirmDialog } from "@/components/confirm-dialog"
import { FormSection } from "@/components/form-section"
import { usePageName } from "@/components/page-title"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Field, FieldContent, FieldDescription, FieldLabel, FieldTitle } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { TargetsField } from "@/features/servers/targets-field"
import { type Area, catalogQuery, type Group, type GroupInput, groupQuery, useDeleteGroup, usersQuery, useSaveGroup } from "./api"
import type { Permission } from "./permissions"
import { useAccess } from "./use-access"

const route = getRouteApi("/_app/settings/groups/$groupId")

const emptyGroup: GroupInput = { name: "", description: "", permissions: [], allServers: true, targets: [] }

export function NewGroupPage() {
  return <GroupEditor initial={emptyGroup} />
}

export function GroupPage() {
  const { groupId } = route.useParams()
  const { data: group, isPending, error } = useQuery(groupQuery(groupId))
  if (isPending) return <Skeleton className="h-96 rounded-2xl" />
  if (error) return <ErrorCallout error={error} />
  // Remounting on save resets the form to what the master stored.
  return <GroupEditor key={JSON.stringify(group)} group={group} initial={group} />
}

function GroupEditor({ group, initial }: { group?: Group; initial: GroupInput }) {
  const { can } = useAccess()
  const { data: catalog } = useQuery(catalogQuery)
  const { data: users } = useQuery({ ...usersQuery, enabled: !!group })
  const navigate = useNavigate()
  const save = useSaveGroup(group?.id)
  const remove = useDeleteGroup()
  usePageName(group?.name ?? t("New group"))
  const start = {
    name: initial.name,
    description: initial.description,
    permissions: initial.permissions,
    allServers: initial.allServers,
    targets: initial.targets,
  }
  const [form, setForm] = useState<GroupInput>(start)
  const editable = can("groups.manage") && !group?.builtin
  const dirty = JSON.stringify(form) !== JSON.stringify(start)
  const blocker = useBlocker({ shouldBlockFn: () => dirty && !save.isPending, enableBeforeUnload: () => dirty, withResolver: true })
  const set = (change: Partial<GroupInput>) => setForm({ ...form, ...change })
  const members = users?.filter((u) => group?.members.includes(u.id)) ?? []

  function submit(event: FormEvent) {
    event.preventDefault()
    save.mutate(form, {
      onSuccess: (saved) => {
        toast.success(t("Saved {{name}}", { name: saved.name }))
        if (!group) void navigate({ to: "/settings/groups/$groupId", params: { groupId: saved.id }, replace: true })
      },
      onError: (e) => toast.error(e.message),
    })
  }

  return (
    <>
      <BackLink to="/settings/groups">{t("Groups")}</BackLink>
      <form onSubmit={submit} className="surface rounded-2xl px-5 sm:px-8">
        <fieldset disabled={!editable} className="contents">
          <FormSection title={t("Group")}>
            {group?.builtin && (
              <Callout tone="warning" icon={CrownIcon}>
                {t(
                  "Administrators have every permission on all servers, including those that later versions add. You can only change who belongs to this group, on the Users tab.",
                )}
              </Callout>
            )}
            <Field>
              <FieldLabel htmlFor="group-name">{t("Name")}</FieldLabel>
              <Input id="group-name" required maxLength={64} value={form.name} onChange={(e) => set({ name: e.target.value })} />
            </Field>
            <Field>
              <FieldLabel htmlFor="group-description">{t("Description")}</FieldLabel>
              <Textarea
                id="group-description"
                rows={2}
                maxLength={500}
                value={form.description}
                onChange={(e) => set({ description: e.target.value })}
              />
            </Field>
          </FormSection>

          {!group?.builtin && (
            <>
              <FormSection title={t("Scope")}>
                <RadioGroup
                  value={form.allServers ? "all" : "some"}
                  onValueChange={(v) => set({ allServers: v === "all" })}
                  aria-label={t("Scope")}
                  className="gap-3 sm:grid-cols-2"
                >
                  <ScopeOption
                    value="all"
                    icon={GlobeIcon}
                    title={t("All servers")}
                    description={t("On every node, including nodes and servers added later.")}
                  />
                  <ScopeOption
                    value="some"
                    icon={TargetIcon}
                    title={t("Selected nodes and servers")}
                    description={t("Whole nodes include servers created later.")}
                  />
                </RadioGroup>
                {!form.allServers && <TargetsField value={form.targets} onChange={(targets) => set({ targets })} />}
              </FormSection>
              <FormSection title={t("Permissions")}>
                {catalog ? (
                  <PermissionsField
                    catalog={catalog}
                    value={form.permissions}
                    onChange={(permissions) => set({ permissions })}
                    scoped={!form.allServers}
                  />
                ) : (
                  <Skeleton className="h-96 rounded-xl" />
                )}
              </FormSection>
            </>
          )}
        </fieldset>

        {group && (
          <FormSection title={t("Members")}>
            <div className="flex flex-wrap gap-2">
              {members.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("No members yet.")}</p>
              ) : (
                members.map((u) => <Chip key={u.id}>{u.username}</Chip>)
              )}
            </div>
          </FormSection>
        )}

        {editable && (
          <div className="-mx-5 flex flex-wrap items-center justify-between gap-3 rounded-b-2xl bg-muted/50 px-5 py-4 sm:-mx-8 sm:px-8">
            {group ? (
              <ConfirmDialog
                trigger={
                  <Button type="button" variant="ghost" className="text-destructive">
                    <TrashIcon />
                    {t("Delete group")}
                  </Button>
                }
                title={t("Delete {{name}}?", { name: group.name })}
                description={t("Its members lose the permissions they only have through this group.")}
                action={t("Delete group")}
                destructive
                onConfirm={() =>
                  remove.mutate(group.id, {
                    onSuccess: () => {
                      toast.success(t("Deleted {{name}}", { name: group.name }))
                      void navigate({ to: "/settings/groups" })
                    },
                    onError: (e) => toast.error(e.message),
                  })
                }
              />
            ) : (
              <span />
            )}
            <Button type="submit" disabled={(!!group && !dirty) || save.isPending}>
              {save.isPending ? t("Saving…") : group ? t("Save group") : t("Create group")}
            </Button>
          </div>
        )}
        <ConfirmDialog
          open={blocker.status === "blocked"}
          onOpenChange={(open) => !open && blocker.reset?.()}
          title={t("Discard your changes?")}
          description={t("Your changes to the group haven't been saved.")}
          action={t("Discard changes")}
          destructive
          onConfirm={() => blocker.proceed?.()}
        />
      </form>
    </>
  )
}

function ScopeOption({
  value,
  icon: Icon,
  title,
  description,
}: {
  value: string
  icon: typeof GlobeIcon
  title: string
  description: string
}) {
  return (
    <FieldLabel htmlFor={`scope-${value}`}>
      <Field orientation="horizontal" className="items-start">
        <Icon className="mt-0.5 size-5 shrink-0 text-violet" weight="duotone" />
        <FieldContent>
          <FieldTitle>{title}</FieldTitle>
          <FieldDescription>{description}</FieldDescription>
        </FieldContent>
        <RadioGroupItem id={`scope-${value}`} value={value} />
      </Field>
    </FieldLabel>
  )
}

/**
 * The permissions by area. Choosing one adds the ones it requires; removing one removes those
 * requiring it, so a group never has a permission without what it needs.
 */
function PermissionsField({
  catalog,
  value,
  onChange,
  scoped,
}: {
  catalog: Area[]
  value: Permission[]
  onChange: (permissions: Permission[]) => void
  scoped: boolean
}) {
  const requires = new Map(catalog.flatMap((a) => a.permissions).map((p) => [p.id, p.requires ?? []]))
  const withRequired = (ids: Permission[]) => {
    const all = new Set(ids)
    for (const id of all) for (const r of requires.get(id) ?? []) all.add(r)
    return [...all]
  }
  const add = (ids: Permission[]) => onChange(withRequired([...value, ...ids]))
  const remove = (ids: Permission[]) => onChange(value.filter((p) => !withRequired([p]).some((r) => ids.includes(r))))

  return (
    <div className="grid gap-6">
      {catalog.map((area) => {
        const ids = area.permissions.map((p) => p.id)
        const all = ids.every((id) => value.includes(id))
        return (
          <fieldset key={area.name} className="grid gap-2">
            <div className="flex items-center justify-between gap-3">
              <legend className="text-sm font-semibold">{area.name}</legend>
              <Button type="button" variant="ghost" size="xs" onClick={() => (all ? remove(ids) : add(ids))}>
                {all ? t("Clear") : t("Choose all")}
              </Button>
            </div>
            <div className="grid gap-2 sm:grid-cols-2">
              {area.permissions.map((p) => (
                <label
                  key={p.id}
                  className="flex cursor-pointer items-start gap-3 rounded-lg p-3 ring-1 ring-foreground/8 hover:bg-muted/50 has-disabled:cursor-default has-data-[state=checked]:bg-primary/5 has-data-[state=checked]:ring-primary/30"
                >
                  <Checkbox
                    className="mt-0.5"
                    checked={value.includes(p.id)}
                    onCheckedChange={(on) => (on === true ? add([p.id]) : remove([p.id]))}
                  />
                  <span className="min-w-0 space-y-0.5">
                    <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm font-medium">
                      {p.label}
                      {scoped && !p.scoped && <Pill tone="neutral">{t("Everywhere")}</Pill>}
                    </span>
                    {p.description && <span className="block text-xs text-muted-foreground">{p.description}</span>}
                  </span>
                </label>
              ))}
            </div>
          </fieldset>
        )
      })}
    </div>
  )
}
