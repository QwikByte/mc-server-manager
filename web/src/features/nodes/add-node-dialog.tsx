import { PlusIcon } from "@phosphor-icons/react"
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
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { useCreateNode } from "./api"
import { EnrollSteps } from "./enroll-steps"

const empty = { name: "", address: "" }

export function AddNodeDialog() {
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState(empty)
  const create = useCreateNode()

  function onOpenChange(next: boolean) {
    setOpen(next)
    if (!next) {
      create.reset()
      setForm(empty)
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    create.mutate(form)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button>
          <PlusIcon />
          Add node
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        {create.data ? (
          <>
            <DialogHeader>
              <DialogTitle>Connect {create.data.node.name}</DialogTitle>
              <DialogDescription>The node was added. Connect its agent to start hosting servers.</DialogDescription>
            </DialogHeader>
            <EnrollSteps joinToken={create.data.joinToken} />
            <DialogFooter>
              <DialogClose asChild>
                <Button>Done</Button>
              </DialogClose>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={submit} className="grid gap-6">
            <DialogHeader>
              <DialogTitle>Add node</DialogTitle>
              <DialogDescription>Register a machine that will run Minecraft servers.</DialogDescription>
            </DialogHeader>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="node-name">Name</FieldLabel>
                <Input
                  id="node-name"
                  placeholder="Frankfurt 1"
                  required
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="node-address">Agent address</FieldLabel>
                <Input
                  id="node-address"
                  placeholder="203.0.113.10:7443"
                  className="font-mono"
                  required
                  value={form.address}
                  onChange={(e) => setForm({ ...form, address: e.target.value })}
                />
                <FieldDescription>Host and port the master uses to reach the agent. The agent listens on port 7443.</FieldDescription>
              </Field>
              {create.error && <FieldError>{create.error.message}</FieldError>}
            </FieldGroup>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="outline">Cancel</Button>
              </DialogClose>
              <Button type="submit" disabled={create.isPending}>
                {create.isPending ? "Adding…" : "Add node"}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  )
}
