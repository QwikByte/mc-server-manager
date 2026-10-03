import { useQuery } from "@tanstack/react-query"
import { getRouteApi, useNavigate } from "@tanstack/react-router"
import { t } from "i18next"
import { toast } from "sonner"
import { BackLink } from "@/components/back-link"
import { ErrorCallout } from "@/components/callout"
import { PageHeader } from "@/components/page-header"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { serverLook } from "@/features/servers/server-types"
import { draftOf, emptyTemplate, templateQuery, useSaveTemplate } from "./api"
import { TemplateForm } from "./template-form"

const route = getRouteApi("/_app/templates/$templateId")

export function TemplatePage() {
  const { can } = useAccess()
  const { templateId } = route.useParams()
  const { data: template, isPending, error } = useQuery(templateQuery(templateId))
  const save = useSaveTemplate(templateId)

  return (
    <>
      <BackLink to="/templates">{t("Templates")}</BackLink>
      {isPending ? (
        <Skeleton className="h-96 rounded-xl" />
      ) : error ? (
        <ErrorCallout error={error} />
      ) : (
        <>
          <PageHeader {...serverLook(template.type)} title={template.name} description={template.description || undefined} />
          {/* Without the permission to manage templates, the template is only shown. */}
          <fieldset disabled={!can("templates.manage")} className="contents">
            <TemplateForm
              // Remounting on save resets the form to what was stored.
              key={JSON.stringify(template)}
              initial={draftOf(template)}
              submitLabel={t("Save template")}
              pending={save.isPending}
              error={save.error}
              onSubmit={(input) => save.mutate(input, { onSuccess: (saved) => toast.success(t("Saved {{name}}", { name: saved.name })) })}
            />
          </fieldset>
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
      <BackLink to="/templates">{t("Templates")}</BackLink>
      <PageHeader {...serverLook("paper")} title={t("New template")} />
      <TemplateForm
        initial={emptyTemplate}
        submitLabel={t("Create template")}
        pending={save.isPending}
        error={save.error}
        onSubmit={(input) =>
          save.mutate(input, {
            onSuccess: (created) => {
              toast.success(t("Created {{name}}", { name: created.name }))
              void navigate({ to: "/templates", ignoreBlocker: true })
            },
          })
        }
      />
    </>
  )
}
