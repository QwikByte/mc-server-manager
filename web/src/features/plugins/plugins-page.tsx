import { PuzzlePieceIcon } from "@phosphor-icons/react"
import { getRouteApi } from "@tanstack/react-router"
import { t } from "i18next"
import { PageHeader } from "@/components/page-header"
import { Segmented } from "@/components/segmented"
import { InstallDialog } from "./install-dialog"
import { PluginSearch } from "./plugin-search"

const route = getRouteApi("/_app/plugins")

/** Finds plugins or mods on Modrinth, or plugins on Hangar, and installs them on several servers at once. */
export function PluginsPage() {
  const { kind = "plugins" } = route.useSearch()
  const navigate = route.useNavigate()
  return (
    <>
      <PageHeader
        icon={PuzzlePieceIcon}
        tone="warning"
        title={t("Plugins & mods")}
        actions={
          <Segmented
            label={t("Show")}
            value={kind}
            onChange={(next) => navigate({ search: { kind: next === "mods" ? next : undefined }, replace: true })}
            options={[
              { value: "plugins", label: t("Plugins") },
              { value: "mods", label: t("Mods") },
            ]}
          />
        }
      />
      <div className="surface rounded-2xl p-4 sm:p-6">
        {/* Each kind starts with its own filters. */}
        <PluginSearch key={kind} kind={kind} action={(hit) => <InstallDialog hit={hit} />} />
      </div>
    </>
  )
}
