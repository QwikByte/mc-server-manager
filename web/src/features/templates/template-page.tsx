import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useNavigate } from "@tanstack/react-router"
import { toast } from "sonner"
import { BackLink } from "@/components/back-link"
import { ErrorCallout } from "@/components/callout"
import { PageHeader } from "@/components/page-header"
import { Skeleton } from "@/components/ui/skeleton"
import { serverLook } from "@/features/servers/server-types"
import { draftOf, emptyTemplate, templateQuery, useSaveTemplate } from "./api"
import { TemplateForm } from "./template-form"

const route = getRouteApi("/_app/templates/$templateId")

export function TemplatePage() {
  const { templateId } = route.useParams()
  const { data: template, isPending, error } = useQuery(templateQuery(templateId))
  const save = useSaveTemplate(templateId)

  return (
    <>
      <BackLink to="/templates">Templates</BackLink>
      {isPending ? (
        <Skeleton className="h-96 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : (
        <>
          <PageHeader {...serverLook(template.type)} title={template.name} description={template.description || undefined} />
          <TemplateForm
            // Remounting on save resets the form to what was stored.
            key={JSON.stringify(template)}
            initial={draftOf(template)}
            submitLabel="Save template"
            pending={save.isPending}
            error={save.error}
            onSubmit={(input) => save.mutate(input, { onSuccess: (t) => toast.success(`Saved ${t.name}`) })}
          />
        </>
      )}
    </>
  )
}

export function NewTemplatePage() {
  const save = useSaveTemplate()
  const navigate = useNavigate()
  return (
    <>
      <BackLink to="/templates">Templates</BackLink>
      <PageHeader {...serverLook("paper")} title="New template" description="Set up what new servers start with." />
      <TemplateForm
        initial={emptyTemplate}
        submitLabel="Create template"
        pending={save.isPending}
        error={save.error}
        onSubmit={(input) =>
          save.mutate(input, {
            onSuccess: (t) => {
              toast.success(`Created ${t.name}`)
              void navigate({ to: "/templates", ignoreBlocker: true })
            },
          })
        }
      />
    </>
  )
}
