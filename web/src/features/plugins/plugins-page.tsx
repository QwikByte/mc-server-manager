import { getRouteApi } from "@tanstack/react-router"
import { t } from "i18next"
import { TabIntro } from "@/components/hub-layout"
import { Segmented } from "@/components/segmented"
import { InstallDialog } from "./install-dialog"
import { InstalledTab } from "./installed-tab"
import { PluginSearch } from "./plugin-search"

const route = getRouteApi("/_app/_library/plugins")

/**
 * Finds plugins or mods on Modrinth, or plugins on Hangar, and installs them on several servers at once; or shows what
 * all servers have installed, and updates or removes it on all of them.
 */
export function PluginsPage() {
  const { kind = "plugins", view = "search" } = route.useSearch()
  const navigate = route.useNavigate()
  return (
    <>
      <TabIntro
        actions={
          <>
            <Segmented
              label={t("View")}
              value={view}
              onChange={(next) => navigate({ search: (s) => ({ ...s, view: next === "installed" ? next : undefined }), replace: true })}
              options={[
                { value: "search", label: t("Find") },
                { value: "installed", label: t("Installed") },
              ]}
            />
            <Segmented
              label={t("Show")}
              value={kind}
              onChange={(next) => navigate({ search: (s) => ({ ...s, kind: next === "mods" ? next : undefined }), replace: true })}
              options={[
                { value: "plugins", label: t("Plugins") },
                { value: "mods", label: t("Mods") },
              ]}
            />
          </>
        }
      >
        {view === "installed"
          ? t("What your servers have installed from Modrinth and Hangar, with updates. Update or remove a project on all of them at once.")
          : t("Search Modrinth and Hangar, and install plugins or mods on several servers at once.")}
      </TabIntro>
      {view === "installed" ? (
        <InstalledTab kind={kind} />
      ) : (
        <div className="surface rounded-xl p-4 sm:p-6">
          {/* Each kind starts with its own filters. */}
          <PluginSearch key={kind} kind={kind} action={(hit) => <InstallDialog hit={hit} />} />
        </div>
      )}
    </>
  )
}
