import { UserPlusIcon } from "@phosphor-icons/react"
import { type FormEvent, useState } from "react"
import { Button } from "@/components/ui/button"
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
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { type Group, useInviteUser } from "./api"
import { GroupPicker } from "./group-picker"
import { SetupLinkView } from "./setup-link-view"

const empty = { username: "", groups: [] as string[] }

/** Adds a user, who sets a password with the setup link shown afterwards. */
export function InviteDialog({ groups }: { groups: Group[] }) {
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState(empty)
  const invite = useInviteUser()

  function onOpenChange(next: boolean) {
    setOpen(next)
    if (!next) {
      invite.reset()
      setForm(empty)
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    invite.mutate(form)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button>
          <UserPlusIcon />
          Invite user
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        {invite.data ? (
          <>
            <DialogHeader>
              <DialogTitle>Invited {invite.data.user.username}</DialogTitle>
              <DialogDescription>The user can sign in after setting a password.</DialogDescription>
            </DialogHeader>
            <SetupLinkView username={invite.data.user.username} link={invite.data.setupLink} />
            <DialogFooter>
              <DialogClose asChild>
                <Button>Done</Button>
              </DialogClose>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={submit} className="grid gap-6">
            <DialogHeader>
              <DialogTitle>Invite user</DialogTitle>
              <DialogDescription>You get a link with which the user sets a password. You never see it.</DialogDescription>
            </DialogHeader>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="invite-username">Username</FieldLabel>
                <Input
                  id="invite-username"
                  required
                  autoComplete="off"
                  pattern="[A-Za-z0-9_.\-]{3,32}"
                  value={form.username}
                  onChange={(e) => setForm({ ...form, username: e.target.value })}
                />
                <FieldDescription>3 to 32 letters, digits, &apos;_&apos;, &apos;.&apos; or &apos;-&apos;.</FieldDescription>
              </Field>
              <FieldSet>
                <FieldLegend variant="label">Groups</FieldLegend>
                <FieldDescription>
                  The user gets the permissions of these groups. You can only choose groups within your own permissions.
                </FieldDescription>
                <GroupPicker groups={groups} value={form.groups} onChange={(groups) => setForm({ ...form, groups })} />
              </FieldSet>
              {invite.error && <FieldError>{invite.error.message}</FieldError>}
            </FieldGroup>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="outline">Cancel</Button>
              </DialogClose>
              <Button type="submit" disabled={invite.isPending}>
                {invite.isPending ? "Inviting…" : "Invite"}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  )
}
