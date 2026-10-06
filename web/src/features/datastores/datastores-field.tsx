import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { Checkbox } from "@/components/ui/checkbox"
import { Field, FieldContent, FieldDescription, FieldLabel, FieldLegend, FieldSet, FieldTitle } from "@/components/ui/field"
import { useAccess } from "@/features/access/use-access"
import { networksQuery } from "@/features/networks/api"
import { datastoresQuery } from "./api"
import { engines } from "./labels"

/** Chooses datastores, e.g. those a backup job backs up with all their databases. */
export function DatastoresField({ value, onChange }: { value: string[]; onChange: (ids: string[]) => void }) {
  const { can } = useAccess()
  const { data: datastores = [] } = useQuery({ ...datastoresQuery, enabled: can("datastores.view") })
  const { data: networks = [] } = useQuery({ ...networksQuery, enabled: can("networks.view") })
  if (datastores.length === 0) return null
  return (
    <FieldSet>
      <FieldLegend variant="label">{t("Datastores")}</FieldLegend>
      <FieldDescription>{t("Their databases are backed up as SQL dumps, while they keep running.")}</FieldDescription>
      <div className="grid gap-3 sm:grid-cols-2">
        {datastores.map((ds) => (
          <FieldLabel key={ds.id} htmlFor={`datastore-${ds.id}`}>
            <Field orientation="horizontal">
              <Checkbox
                id={`datastore-${ds.id}`}
                checked={value.includes(ds.id)}
                onCheckedChange={(on) => onChange(on === true ? [...value, ds.id] : value.filter((id) => id !== ds.id))}
              />
              <FieldContent>
                <FieldTitle>
                  <span className="font-mono">{ds.name}</span>
                </FieldTitle>
                <FieldDescription>
                  {[networks.find((n) => n.id === ds.networkId)?.name, `${engines[ds.engine].label} ${ds.version}`, ds.nodeName]
                    .filter(Boolean)
                    .join(" · ")}
                </FieldDescription>
              </FieldContent>
            </Field>
          </FieldLabel>
        ))}
      </div>
    </FieldSet>
  )
}
