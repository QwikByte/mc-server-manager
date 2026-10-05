import { ShieldCheckIcon, ShieldWarningIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { Callout } from "@/components/callout"
import { Checkbox } from "@/components/ui/checkbox"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { cn } from "@/lib/utils"
import type { Forwarding } from "./api"

/** How the proxy tells the servers who a player is: Velocity's modern forwarding, or BungeeCord's. */
export function ForwardingChoice({
  value,
  onChange,
  bungee,
  disabled,
}: {
  value: Forwarding
  onChange: (value: Forwarding) => void
  bungee: boolean
  disabled?: boolean
}) {
  const options = [
    {
      value: "modern" as const,
      icon: ShieldCheckIcon,
      title: t("Modern"),
      badge: t("Recommended"),
      description: t("Velocity signs the identity of each player with a secret that only the network's servers know. Paper, Fabric, Forge, NeoForge and their forks from 1.13 on."),
    },
    {
      value: "legacy" as const,
      icon: ShieldWarningIcon,
      title: bungee ? t("BungeeCord (ip_forward)") : t("Legacy"),
      badge: bungee ? t("BungeeCord's only way") : undefined,
      description: t("Compatible with BungeeCord and Minecraft before 1.13, but unsigned: only the proxy may reach the servers, or anyone can join as any player."),
    },
  ].filter((o) => !bungee || o.value === "legacy")

  return (
    <div role="radiogroup" aria-label={t("Forwarding")} className="grid gap-3 sm:grid-cols-2">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={value === o.value}
          disabled={disabled || bungee}
          onClick={() => onChange(o.value)}
          className={cn(
            "flex gap-3 rounded-xl p-4 text-left ring-1 ring-border transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring",
            "aria-checked:bg-primary/5 aria-checked:ring-2 aria-checked:ring-primary enabled:hover:bg-muted/50 disabled:cursor-default",
          )}
        >
          <o.icon className={cn("mt-0.5 size-5 shrink-0", o.value === "modern" ? "text-success" : "text-warning")} weight="duotone" />
          <span className="space-y-1">
            <span className="flex flex-wrap items-center gap-2 text-sm font-semibold">
              {o.title}
              {o.badge && <span className="rounded-full bg-muted px-2 py-0.5 text-[0.6875rem] font-medium text-muted-foreground">{o.badge}</span>}
            </span>
            <span className="block text-xs text-muted-foreground">{o.description}</span>
          </span>
        </button>
      ))}
    </div>
  )
}

/**
 * Asks for the confirmation that a firewall protects the servers on other nodes, which
 * legacy forwarding needs. Servers on the proxy's node need none: their port isn't published.
 */
export function FirewallConfirmation({
  checked,
  onChange,
  proxyNode,
  disabled,
}: {
  checked: boolean
  onChange: (checked: boolean) => void
  proxyNode?: string
  disabled?: boolean
}) {
  return (
    <Callout tone="warning" icon={ShieldWarningIcon} title={t("Servers on other nodes are reachable from outside")}>
      <p>
        {t("With legacy forwarding, anyone who reaches such a server can join it as any player, also as an operator. Docker bypasses firewalls such as ufw, so add a rule to Docker's DOCKER-USER chain on each of these nodes, as shown with the servers.")}
      </p>
      <Field orientation="horizontal" className="mt-3">
        <Checkbox id="firewalled" checked={checked} disabled={disabled} onCheckedChange={(on) => onChange(on === true)} />
        <div>
          <FieldLabel htmlFor="firewalled">
            {proxyNode
              ? t("A firewall lets only {{node}} reach these servers", { node: proxyNode })
              : t("A firewall lets only the proxy's node reach these servers")}
          </FieldLabel>
          <FieldDescription>{t("Servers on the proxy's own node need nothing: only the proxy reaches them.")}</FieldDescription>
        </div>
      </Field>
    </Callout>
  )
}
