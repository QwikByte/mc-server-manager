import { DeviceMobileIcon, WarningIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { Callout } from "@/components/callout"
import { Section } from "@/components/section"
import { Field, FieldContent, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import type { Node } from "@/features/nodes/api"
import { gameVersionsQuery, versionsQuery } from "@/features/plugins/api"
import type { NodeServer } from "@/features/servers/api"
import { suggestPort } from "@/features/servers/server-types"
import type { Network } from "./api"
import type { Draft } from "./draft"
import { bedrockPortError } from "./problems"
import { findServer } from "./servers"

/** The port Bedrock players use unless it is taken, Geyser's default. */
const bedrockDefault = 19132
/** Geyser's project on Modrinth, whose newest version tells which Minecraft it joins with. */
const geyser = "wKkoqHrH"
/** The memory in MB a proxy needs with Geyser, which keeps the worlds of Bedrock players. */
const geyserMemory = 1024

/**
 * Lets players of the Bedrock Edition join through Geyser and Floodgate on the proxy, at a
 * UDP port of the proxy's node. It tells where they connect and which servers need ViaVersion.
 */
export function BedrockSection({
  network,
  draft,
  onChange,
  servers,
  proxyNode,
  proxyHost,
  editable,
}: {
  network: Network
  draft: Draft
  onChange: (change: Partial<Draft>) => void
  servers?: NodeServer[]
  proxyNode?: Node
  proxyHost?: string
  editable: boolean
}) {
  const port = draft.bedrockPort
  const on = port !== 0
  const { data: releases = [] } = useQuery(gameVersionsQuery)
  const { data: geyserVersions } = useQuery({ ...versionsQuery(geyser, network.proxyType, ""), enabled: on })
  const error = bedrockPortError(port)
  // Ports of the proxy's node, except the one its proxy has for Bedrock players already.
  const used = (servers ?? [])
    .filter((s) => s.nodeId === network.proxy.nodeId)
    .flatMap((s) => (s.bedrockPort && s.id !== network.proxy.serverId ? [s.port, s.bedrockPort] : [s.port]))
  const suggested =
    network.bedrockPort || suggestPort(used, bedrockDefault, proxyNode?.portMin ?? undefined, proxyNode?.portMax ?? undefined)
  // Geyser joins with a version of Minecraft: servers of older versions need ViaVersion, and newer ones also ViaBackwards.
  const joins = geyserVersions?.[0]?.gameVersions ?? []
  const newest = Math.min(...joins.map((v) => releases.indexOf(v)).filter((i) => i >= 0))
  const others = draft.backends.flatMap((b) => {
    const version = findServer(servers, b)?.version
    if (!version || joins.length === 0 || joins.includes(version)) return []
    // Which version follows new releases depends on the software, e.g. Paper waits for a stable build.
    if (version === "LATEST") return releases[0] && !joins.includes(releases[0]) ? [{ name: b.name, kind: "latest" }] : []
    const i = releases.indexOf(version)
    return i < 0 ? [] : [{ name: `${b.name} (${version})`, kind: i < newest ? "newer" : "older" }]
  })
  const proxy = findServer(servers, network.proxy)
  const list = (kind: string) =>
    others
      .filter((o) => o.kind === kind)
      .map((o) => o.name)
      .join(", ")

  return (
    <Section title={t("Bedrock Edition")} description={t("Players on phones, consoles and Windows join through Geyser on the proxy.")}>
      <div className="surface grid gap-6 rounded-xl p-5 sm:p-6">
        <Field orientation="horizontal">
          <Switch id="bedrock" checked={on} disabled={!editable} onCheckedChange={(v) => onChange({ bedrockPort: v ? suggested : 0 })} />
          <FieldContent>
            <FieldLabel htmlFor="bedrock">{t("Let Bedrock players join")}</FieldLabel>
            <FieldDescription>
              {t(
                "Geyser translates their game and Floodgate lets them join without a Java account. The panel installs both on the proxy and updates them whenever the network is applied.",
              )}
            </FieldDescription>
          </FieldContent>
        </Field>
        {on && (
          <>
            <Field data-invalid={!!error}>
              <FieldLabel htmlFor="bedrock-port">{t("Port (UDP)")}</FieldLabel>
              <Input
                id="bedrock-port"
                type="number"
                min={1024}
                max={65535}
                className="font-mono sm:max-w-48"
                disabled={!editable}
                value={Number.isNaN(port) ? "" : port}
                aria-invalid={!!error}
                onChange={(e) => onChange({ bedrockPort: e.target.valueAsNumber })}
              />
              {error && <FieldError>{error}</FieldError>}
            </Field>
            {!error && (
              <Callout
                icon={DeviceMobileIcon}
                title={
                  proxyHost &&
                  t("Bedrock players connect to {{address}}", {
                    address: `${proxyHost}:${port}`,
                  })
                }
              >
                {proxyNode
                  ? t("Open UDP port {{port}} on {{node}} for them, e.g. in the firewall of the hosting provider.", {
                      port,
                      node: proxyNode.name,
                    })
                  : t("Open UDP port {{port}} on the proxy's node for them, e.g. in the firewall of the hosting provider.", { port })}{" "}
                {t("Their names start with a dot, e.g. .Steve on the whitelist.")}
              </Callout>
            )}
            {proxy && proxy.memoryMb < geyserMemory && (
              <Callout tone="warning" icon={WarningIcon} title={t("The proxy needs more memory")}>
                {t("With Geyser, it needs at least {{memory}} MB, and more the more Bedrock players join, but it has {{current}} MB.", {
                  memory: geyserMemory,
                  current: proxy.memoryMb,
                })}{" "}
                <Link
                  to="/nodes/$nodeId/servers/$serverId/settings"
                  params={network.proxy}
                  className="font-medium text-foreground hover:underline"
                >
                  {t("Change it in the settings of the proxy")}
                </Link>
              </Callout>
            )}
            {others.length > 0 && (
              <Callout tone="warning" icon={WarningIcon} title={t("Some servers need ViaVersion")}>
                {[
                  t("Geyser joins as Minecraft {{version}}.", {
                    version: joins.join(", "),
                  }),
                  list("older") && t("Install ViaVersion on {{names}} so that Bedrock players can join them.", { names: list("older") }),
                  list("newer") && t("Install ViaVersion and ViaBackwards on {{names}}, which are newer.", { names: list("newer") }),
                  list("latest") &&
                    t("{{names}} follow new releases and need ViaVersion and ViaBackwards while they run a newer version than Geyser.", {
                      names: list("latest"),
                    }),
                ]
                  .filter(Boolean)
                  .join(" ")}
              </Callout>
            )}
          </>
        )}
      </div>
    </Section>
  )
}
