import { t } from "i18next"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import type { NodeServer } from "@/features/servers/api"
import type { ServerRef } from "./api"
import { key, refOf } from "./servers"

/** Picks one of the given servers, which are labelled with their node. */
export function ServerSelect({
  id,
  servers,
  value,
  onChange,
  placeholder,
}: {
  id: string
  servers: NodeServer[]
  value?: ServerRef
  onChange: (ref: ServerRef) => void
  placeholder: string
}) {
  return (
    <Select
      value={value ? key(value) : ""}
      onValueChange={(v) => {
        const server = servers.find((s) => key(refOf(s)) === v)
        if (server) onChange(refOf(server))
      }}
      disabled={servers.length === 0}
    >
      <SelectTrigger id={id} className="w-full">
        <SelectValue placeholder={servers.length === 0 ? t("No suitable server available") : placeholder} />
      </SelectTrigger>
      <SelectContent>
        {servers.map((s) => (
          <SelectItem key={key(refOf(s))} value={key(refOf(s))}>
            {s.name}
            <span className="text-muted-foreground">{t("{{node}} · port {{port}}", { node: s.nodeName, port: s.port })}</span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
