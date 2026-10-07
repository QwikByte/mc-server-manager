import { CreateNetworkDialog } from "@/features/networks/create-network-dialog"
import { AddNodeDialog } from "@/features/nodes/add-node-dialog"
import { CreateServerDialog } from "@/features/servers/create-server-dialog"
import type { Creation } from "./shortcuts"

const dialogs = { server: CreateServerDialog, network: CreateNetworkDialog, node: AddNodeDialog }

/** The dialog that creates what the palette or a shortcut chose, without the button that usually opens it. */
export function CreateDialog({ what, onClose }: { what: Creation; onClose: () => void }) {
  const Chosen = dialogs[what]
  return <Chosen open onOpenChange={(open) => !open && onClose()} />
}
