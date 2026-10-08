import type { Network, ServerRef } from "@/features/networks/api"
import type { PlayerAction } from "./api"
import { PlayerMessageDialog } from "./message-dialog"
import { PlayerActionDialog, type Scope } from "./player-action-dialog"
import { SendDialog } from "./send-dialog"

/**
 * What a page about players asks of a dialog: an action on players (without names, the dialog asks for them), sending
 * one to another server of a network, or a message to players on the servers they're on.
 */
export type PlayerDialog =
  | { action: PlayerAction; names?: string[]; scopes: Scope[] }
  | { send: string; network: Network; from?: string }
  | { message: string[]; servers: ServerRef[] }

/** The dialog a page about players asks for, if any. */
export function PlayerDialogs({ dialog, onClose }: { dialog?: PlayerDialog; onClose: () => void }) {
  if (!dialog) return null
  if ("send" in dialog) return <SendDialog name={dialog.send} network={dialog.network} from={dialog.from} onClose={onClose} />
  if ("message" in dialog) return <PlayerMessageDialog names={dialog.message} servers={dialog.servers} onClose={onClose} />
  return <PlayerActionDialog action={dialog.action} names={dialog.names} scopes={dialog.scopes} onClose={onClose} />
}
