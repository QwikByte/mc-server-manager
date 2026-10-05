import { t } from "i18next"
import type { Backend, ForcedHost, Network } from "./api"
import type { Draft } from "./draft"
import { routeOf } from "./servers"

// The same rules as the master's, which checks them again.
const namePattern = /^[a-z0-9][a-z0-9_-]{0,31}$/
const hostPattern = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$/

/** Why the name of a server can't be used, if it can't. */
export function nameError(backends: Backend[], i: number): string | undefined {
  const { name } = backends[i]
  if (!namePattern.test(name)) return t("Use up to 32 lower-case letters, digits, - and _.")
  if (name === "try") return t("Velocity reserves try.")
  if (backends.findIndex((b) => b.name === name) !== i) return t("Another server has this name.")
}

/** Why a forced host can't be used, if it can't. */
export function hostError(hosts: ForcedHost[], i: number): string | undefined {
  const { host, servers } = hosts[i]
  if (!hostPattern.test(host) || host.length > 253) return t("Enter a host name such as survival.example.com, without a port.")
  if (hosts.findIndex((h) => h.host === host) !== i) return t("This host name is there already.")
  if (servers.length === 0) return t("Choose a server for this host name.")
}

/**
 * Whether servers on other nodes than the proxy's can be reached by others, which legacy forwarding can't tell apart from the
 * proxy: unless the private network of the nodes lets only the proxy reach them.
 */
export function exposed(network: Network, draft: Draft, isPrivate: (a: string, b: string) => boolean) {
  return draft.forwarding === "legacy" && draft.backends.some((b) => routeOf(network, b, isPrivate) === "public")
}

/** Why the Bedrock port can't be used, if it can't; the master also checks that it is free. */
export function bedrockPortError(port: number): string | undefined {
  if (port !== 0 && !(Number.isInteger(port) && port >= 1024 && port <= 65535)) return t("Choose a port from 1024 to 65535.")
}

/** Whether the draft of a network can be saved. */
export function isValid(network: Network, draft: Draft, isPrivate: (a: string, b: string) => boolean) {
  return (
    draft.name.trim() !== "" &&
    draft.backends.length > 0 &&
    draft.try.length > 0 &&
    draft.backends.every((b, i) => !nameError(draft.backends, i) && b.motd.length <= 256) &&
    draft.forcedHosts.every((_, i) => !hostError(draft.forcedHosts, i)) &&
    (!exposed(network, draft, isPrivate) || draft.firewalled) &&
    !bedrockPortError(draft.bedrockPort)
  )
}
