import { PuzzlePieceIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { PageHeader } from "@/components/page-header"
import { InstallDialog } from "./install-dialog"
import { PluginSearch } from "./plugin-search"

/** Finds plugins and mods on Modrinth and installs them on several servers at once. */
export function PluginsPage() {
  return (
    <>
      <PageHeader icon={PuzzlePieceIcon} tone="warning" title={t("Plugins & mods")} />
      <div className="surface rounded-2xl p-4 sm:p-6">
        <PluginSearch action={(hit) => <InstallDialog hit={hit} />} />
      </div>
    </>
  )
}
