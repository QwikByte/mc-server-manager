import { useQuery } from "@tanstack/react-query"
import { useMemo } from "react"
import { accessQuery, type Grants } from "./api"
import type { Permission } from "./permissions"

export interface Access {
  /** Whether p applies everywhere, to a whole node, or to a server. */
  can: (p: Permission, nodeId?: string, serverId?: string) => boolean
  /** Whether p applies anywhere, or anywhere on a node. */
  canSomewhere: (p: Permission, nodeId?: string) => boolean
  /** Whether the user is in the Administrators group, for what no other group may get, e.g. updates. */
  admin: boolean
}

/** The permissions of the signed-in user, which the panel uses to only offer what is allowed. The master checks them anyway. */
export function useAccess(): Access {
  const { data } = useQuery(accessQuery)
  return useMemo(() => accessOf(data), [data])
}

export function accessOf(grants?: Grants): Access {
  const scope = (p: Permission) => (grants?.admin ? { all: true, nodes: [], servers: [] } : grants?.permissions[p])
  return {
    admin: !!grants?.admin,
    can: (p, nodeId, serverId) => {
      const s = scope(p)
      if (!s || s.all || !nodeId) return !!s?.all
      return s.nodes.includes(nodeId) || (!!serverId && s.servers.some((t) => t.nodeId === nodeId && t.serverId === serverId))
    },
    canSomewhere: (p, nodeId) => {
      const s = scope(p)
      if (!s || s.all || !nodeId) return !!s && (s.all || s.nodes.length > 0 || s.servers.length > 0)
      return s.nodes.includes(nodeId) || s.servers.some((t) => t.nodeId === nodeId)
    },
  }
}
