import { WarningIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { Callout } from "@/components/callout"
import { SelectGroup, SelectItem, SelectLabel, SelectSeparator } from "@/components/ui/select"
import { serverType, serverTypes } from "./server-types"

/** The software a new server or template can run, game servers first. Software at its end of life is only listed while it is chosen, e.g. by a template. */
export function SoftwareOptions({ chosen }: { chosen: string }) {
  return [false, true].map((proxy) => (
    <SelectGroup key={String(proxy)}>
      {proxy && <SelectSeparator />}
      <SelectLabel>{proxy ? t("Proxies for networks") : t("Game servers")}</SelectLabel>
      {serverTypes
        .filter((s) => s.proxy === proxy && (!s.endOfLife || s.value === chosen))
        .map((s) => (
          <SelectItem key={s.value} value={s.value}>
            {s.label}
          </SelectItem>
        ))}
    </SelectGroup>
  ))
}

/** Warns that software reached its end of life, and tells what to use instead. */
export function EndOfLifeNotice({ type, className }: { type: string; className?: string }) {
  const { label, endOfLife } = serverType(type)
  if (!endOfLife) return null
  return (
    <Callout tone="warning" icon={WarningIcon} role="note" className={className} title={t("{{software}} reached its end of life", { software: label })}>
      {t(endOfLife)}
    </Callout>
  )
}
