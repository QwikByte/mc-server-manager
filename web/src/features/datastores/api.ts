import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import type { Result as FileSetResult } from "@/features/filesets/api"
import { type Operation, operate } from "@/features/operations/api"
import { api } from "@/lib/api"

export type Engine = "mariadb" | "postgres"

export type DatastoreState = "stopped" | "starting" | "running" | "unhealthy" | "unknown"

/** A database of a datastore, with a user of the same name. Its password never leaves the master but to the servers. */
export interface Database {
  name: string
  createdAt: string
}

/** A MariaDB or PostgreSQL server of a network on a node, as its agent reports it. */
export interface Datastore {
  id: string
  networkId: string
  nodeId: string
  nodeName: string
  name: string
  engine: Engine
  version: string
  memoryMb: number
  cpuMillis: number
  storage: string
  /** Published in the private network of the nodes for the servers of other nodes; 0 until one needs it. */
  port: number
  createdAt: string
  databases: Database[]
  state: DatastoreState
  size: number
  /** The version whose data an upgrade left, until it is removed. */
  previous?: string
  /** The major versions of its engine that the agent runs, oldest first. */
  versions: string[]
  /** Databases that the datastore lacks, e.g. after its data was replaced. */
  missing: string[]
  problem?: string
}

/** A server that a file set with fields of a database is for. */
export interface DatabaseUse {
  nodeId: string
  nodeName: string
  serverId: string
  name?: string
  running: boolean
  state: string
  problem?: string
  setId: string
  setName: string
  /** <datastore>.<database> */
  database: string
}

export interface Dump {
  id: string
  label: string
  createdAt: string
  size: number
  location: string
  databases: string[]
  jobId?: string
}

export interface DatastoreInput {
  name: string
  nodeId: string
  engine: Engine
  version: string
  memoryMb: number
  cpuMillis: number
  storage: string
}

export interface DatastoreChange {
  memoryMb?: number
  cpuMillis?: number
  version?: string
  updateImage?: boolean
  removePrevious?: boolean
}

/** The fields of a database that file sets fill in. */
export const fields = ["host", "port", "database", "user", "password"] as const

export const placeholderOf = (datastore: string, database: string, field: (typeof fields)[number]) =>
  `{{datastore:${datastore}.${database}.${field}}}`

export const datastoresQuery = queryOptions({
  queryKey: ["datastores"],
  queryFn: () => api<Datastore[]>("/datastores"),
})

export const networkDatastoresQuery = (networkId: string) =>
  queryOptions({
    queryKey: ["datastores", "network", networkId],
    queryFn: () => api<{ datastores: Datastore[]; uses: DatabaseUse[] }>(`/networks/${networkId}/datastores`),
    refetchInterval: 10_000,
  })

export const dumpsQuery = (id: string) =>
  queryOptions({
    queryKey: ["datastores", id, "backups"],
    queryFn: () => api<Dump[]>(`/datastores/${id}/backups`),
  })

export const downloadUrl = (id: string, dump: string) => `/api/datastores/${id}/backups/${dump}/download`

type Started = { onStart?: (op: Operation) => void }

export function useCreateDatastore(networkId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ onStart, ...input }: DatastoreInput & Started) => operate<Datastore>(`/networks/${networkId}/datastores`, { body: input }, onStart),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["datastores"] }),
  })
}

/** Changes a datastore, its databases and its dumps. */
export function useDatastore(id: string) {
  const queryClient = useQueryClient()
  const onSettled = () => queryClient.invalidateQueries({ queryKey: ["datastores"] })
  const base = `/datastores/${id}`
  return {
    update: useMutation({
      mutationFn: ({ onStart, ...change }: DatastoreChange & Started) => operate<Datastore>(base, { method: "PATCH", body: change }, onStart),
      onSettled,
    }),
    power: useMutation({ mutationFn: (action: "start" | "stop") => api(`${base}/${action}`, { method: "POST" }), onSettled }),
    remove: useMutation({ mutationFn: () => api(base, { method: "DELETE" }), onSettled }),
    addDatabase: useMutation({ mutationFn: (name: string) => api<Database>(`${base}/databases`, { body: { name } }), onSettled }),
    dropDatabase: useMutation({ mutationFn: (name: string) => api(`${base}/databases/${name}`, { method: "DELETE" }), onSettled }),
    rotate: useMutation({
      mutationFn: ({ name, onStart }: { name: string } & Started) =>
        operate<{ results: FileSetResult[] }>(`${base}/databases/${name}/rotate`, { method: "POST" }, onStart),
      onSettled,
    }),
    dump: useMutation({
      mutationFn: ({ onStart, ...input }: { label: string; databases: string[] } & Started) =>
        operate<Dump>(`${base}/backups`, { body: input }, onStart),
      onSettled,
    }),
    restore: useMutation({
      mutationFn: ({ dump, databases, onStart }: { dump: string; databases: string[] } & Started) =>
        operate(`${base}/backups/${dump}/restore`, { body: { databases } }, onStart),
      onSettled: () => {
        void onSettled()
        void queryClient.invalidateQueries({ queryKey: ["nodes"] })
      },
    }),
    removeDump: useMutation({ mutationFn: (dump: string) => api(`${base}/backups/${dump}`, { method: "DELETE" }), onSettled }),
  }
}
