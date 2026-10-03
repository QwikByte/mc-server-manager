import { PuzzlePieceIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { useState } from "react"
import { PageHeader } from "@/components/page-header"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { serverTypes } from "@/features/servers/server-types"
import { InstallDialog } from "./install-dialog"
import { PluginSearch } from "./plugin-search"

const any = "any"

/** Finds plugins and mods on Modrinth and installs them on several servers at once. */
export function PluginsPage() {
  const [type, setType] = useState("paper")
  return (
    <>
      <PageHeader
        icon={PuzzlePieceIcon}
        tone="warning"
        title={t("Plugins & mods")}
        description={t("Install plugins and mods from Modrinth on any number of your servers.")}
        actions={
          <Select value={type} onValueChange={setType}>
            <SelectTrigger aria-label={t("Software")} className="w-44">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={any}>{t("All software")}</SelectItem>
              {serverTypes
                .filter((s) => s.addons)
                .map((s) => (
                  <SelectItem key={s.value} value={s.value}>
                    {s.label}
                  </SelectItem>
                ))}
            </SelectContent>
          </Select>
        }
      />
      <div className="surface rounded-2xl p-4 sm:p-6">
        <PluginSearch type={type === any ? undefined : type} action={(hit) => <InstallDialog hit={hit} />} />
      </div>
    </>
  )
}
