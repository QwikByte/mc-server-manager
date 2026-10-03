import type { ReactNode } from "react"
import { FieldGroup } from "@/components/ui/field"

/** A group of fields in a long form, titled on the left on wide screens. */
export function FormSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="grid gap-x-10 gap-y-5 border-b py-8 lg:grid-cols-[14rem_1fr]" aria-label={title}>
      <h2 className="heading text-base">{title}</h2>
      <FieldGroup>{children}</FieldGroup>
    </section>
  )
}
