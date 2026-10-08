import { DiscordLogoIcon, EnvelopeSimpleIcon, type Icon, SlackLogoIcon, WebhooksLogoIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import type { Tone } from "@/components/tone"
import { categoryLabel } from "@/features/logs/meta"
import { msg } from "@/lib/i18n"
import type { ChannelKind, Rule } from "./api"

/** How the kinds of channels look; the labels are translated with t. */
export const kinds: Record<ChannelKind, { label: string; icon: Icon; tone: Tone; hint: string }> = {
  discord: {
    label: msg("Discord"),
    icon: DiscordLogoIcon,
    tone: "violet",
    hint: msg("In Discord, open the settings of a channel, then Integrations, Webhooks, and copy the URL of a new webhook."),
  },
  slack: {
    label: msg("Slack"),
    icon: SlackLogoIcon,
    tone: "info",
    hint: msg("In Slack, add an app with incoming webhooks to your workspace and copy the URL of a webhook for a channel."),
  },
  webhook: {
    label: msg("Webhook"),
    icon: WebhooksLogoIcon,
    tone: "neutral",
    hint: msg("Gets the entries as JSON by POST. The documentation of the master describes the format."),
  },
  email: { label: msg("Email"), icon: EnvelopeSimpleIcon, tone: "success", hint: msg("Sends mails through your mail server.") },
}

/** The levels a rule can start from; the labels are translated with t. */
export const ruleLevels: { value: Rule["level"]; label: string }[] = [
  { value: "error", label: msg("Errors") },
  { value: "warn", label: msg("Warnings and errors") },
  { value: "info", label: msg("Information, warnings and errors") },
]

export const levelLabel = (level: Rule["level"]) => t(ruleLevels.find((l) => l.value === level)?.label ?? level)

/** The categories of a rule, or that it has all. */
export const describeCategories = (categories: string[]) =>
  categories.length === 0 ? t("All categories") : categories.map(categoryLabel).join(", ")
