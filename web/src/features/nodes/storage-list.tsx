import { HardDriveIcon } from "@phosphor-icons/react"
import { IconTile } from "@/components/icon-tile"
import { Meter } from "@/components/meter"
import { Section } from "@/components/section"
import { formatBytes } from "@/lib/format"
import type { StorageLocation } from "./api"

/** The storage locations of a node with their free space. */
export function StorageList({ locations }: { locations: StorageLocation[] }) {
  return (
    <Section
      title="Storage"
      description={
        <>
          Add a location on the node with{" "}
          <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">mcsm-agent storage add &lt;name&gt; &lt;path&gt;</code>.
        </>
      }
    >
      <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {locations.map((l) => (
          <li key={l.name} className="surface space-y-4 rounded-xl p-4">
            <div className="flex items-center gap-3">
              <IconTile icon={HardDriveIcon} tone="neutral" size="sm" />
              <div className="min-w-0">
                <p className="font-semibold">{l.name}</p>
                <p className="truncate font-mono text-xs text-muted-foreground" title={l.path}>
                  {l.path}
                </p>
              </div>
            </div>
            <div className="space-y-2">
              <Meter value={l.totalBytes ? 1 - l.freeBytes / l.totalBytes : 0} label={`${l.name} used`} />
              <p className="text-xs text-muted-foreground">
                {formatBytes(l.freeBytes)} free of {formatBytes(l.totalBytes)}
              </p>
            </div>
          </li>
        ))}
      </ul>
    </Section>
  )
}
