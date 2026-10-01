import { formatBytes } from "@/lib/format"
import type { StorageLocation } from "./api"

/** The storage locations of a node with their free space. */
export function StorageList({ locations }: { locations: StorageLocation[] }) {
  return (
    <section className="mt-10" aria-labelledby="storage-heading">
      <h2 id="storage-heading" className="heading mb-4 text-xl">
        Storage
      </h2>
      <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {locations.map((l) => {
          const used = l.totalBytes ? 1 - l.freeBytes / l.totalBytes : 0
          return (
            <li key={l.name} className="border bg-card p-4">
              <div className="flex items-baseline justify-between gap-3">
                <span className="font-medium">{l.name}</span>
                <span className="text-xs text-muted-foreground">
                  {formatBytes(l.freeBytes)} free of {formatBytes(l.totalBytes)}
                </span>
              </div>
              <p className="mt-1 truncate font-mono text-xs text-muted-foreground" title={l.path}>
                {l.path}
              </p>
              <div
                className="mt-3 h-1.5 bg-muted"
                role="meter"
                aria-label={`${l.name} used`}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={Math.round(used * 100)}
              >
                <div className={used > 0.9 ? "h-full bg-destructive" : "h-full bg-primary"} style={{ width: `${used * 100}%` }} />
              </div>
            </li>
          )
        })}
      </ul>
      <p className="mt-3 text-sm text-muted-foreground">
        Add a location on the node with <code className="font-mono">mcsm-agent storage add &lt;name&gt; &lt;path&gt;</code>.
      </p>
    </section>
  )
}
