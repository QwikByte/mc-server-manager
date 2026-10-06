import { CubeIcon, GraphIcon, HardDrivesIcon, type Icon } from "@phosphor-icons/react"
import { t } from "i18next"
import type { ComponentProps } from "react"
import { IconTile } from "@/components/icon-tile"
import type { Tone } from "@/components/tone"
import { Button } from "@/components/ui/button"
import { useAccess } from "@/features/access/use-access"
import { CreateNetworkDialog } from "@/features/networks/create-network-dialog"
import { AddNodeDialog } from "@/features/nodes/add-node-dialog"
import { CreateServerDialog } from "@/features/servers/create-server-dialog"
import { Panel } from "./panel"

/** What is created most often, a click away. */
export function QuickActions({ title }: { title: string }) {
  const { can, canSomewhere } = useAccess()
  return (
    <Panel title={title}>
      <div className="grid gap-2 p-3 @md:grid-cols-2 @3xl:grid-cols-3">
        {canSomewhere("servers.create") && (
          <CreateServerDialog trigger={<Action icon={CubeIcon} tone="success" label={t("Create server")} detail={t("From scratch or a template")} />} />
        )}
        {can("networks.manage") && (
          <CreateNetworkDialog trigger={<Action icon={GraphIcon} tone="violet" label={t("Create network")} detail={t("Servers behind a proxy")} />} />
        )}
        {can("nodes.enroll") && (
          <AddNodeDialog trigger={<Action icon={HardDrivesIcon} tone="info" label={t("Add node")} detail={t("Another dedicated server")} />} />
        )}
      </div>
    </Panel>
  )
}

/** A button with an icon and what it does; the dialog's trigger passes its props on. */
function Action({ icon, tone, label, detail, ...props }: ComponentProps<typeof Button> & { icon: Icon; tone: Tone; label: string; detail: string }) {
  return (
    <Button variant="ghost" className="lift h-auto justify-start gap-3 rounded-xl p-3 text-left whitespace-normal hover:shadow-md" {...props}>
      <IconTile icon={icon} tone={tone} size="md" />
      <span className="min-w-0">
        <span className="block font-semibold">{label}</span>
        <span className="block text-xs font-normal text-muted-foreground">{detail}</span>
      </span>
    </Button>
  )
}
