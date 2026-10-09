import { ChatTextIcon, GaugeIcon, type Icon, MoonIcon, ScrollIcon, WebhooksLogoIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { msg } from "@/lib/i18n"
import { type Condition, type Draft, emptyDraft, type Op } from "./api"
import { triggers } from "./catalog"

/** A workflow to start from, which shows what workflows can do. */
export interface Example {
  name: string
  description: string
  icon: Icon
  draft: () => Draft
}

const rule = (left: string, op: Op, right = ""): Condition => ({ match: "all", rules: [{ left, op, right }] })

/** A draft named and described like its example, in the panel's language. */
const draft = (e: Pick<Example, "name" | "description">, change: Partial<Draft>): Draft => ({
  ...emptyDraft,
  name: t(e.name),
  description: t(e.description),
  ...change,
})

const welcome = { name: msg("Welcome players"), description: msg("Greets each player who joins a server, by name.") }
const nightly = {
  name: msg("Restart at night while nobody plays"),
  description: msg("Every night, restarts the running servers without players and tells the others why they keep running."),
}
const crashes = { name: msg("Report crashes"), description: msg("Sends a notification when a server crashes, and starts it again if it stopped.") }
const full = { name: msg("Open more servers when it's full"), description: msg("Starts more servers once a lobby has many players for a few minutes.") }
const hooked = {
  name: msg("Commands from another system"),
  description: msg("A webhook lets e.g. a shop or a bot run a console command with the data it sends."),
}

export const examples: Example[] = [
  {
    ...welcome,
    icon: ChatTextIcon,
    draft: () =>
      draft(welcome, {
        triggers: [{ ...triggers.server.create(), on: "joined" }],
        steps: [
          {
            id: "message_1",
            kind: "message",
            with: {
              targets: [],
              from: "{{trigger.server}}",
              kind: "title",
              text: t("Welcome, {{player}}!", { player: "{{trigger.player}}" }),
              subtitle: t("Have fun on {{server}}", { server: "{{trigger.server.name}}" }),
              player: "{{trigger.player}}",
            },
          },
        ],
      }),
  },
  {
    ...nightly,
    icon: MoonIcon,
    draft: () =>
      draft(nightly, {
        triggers: [triggers.schedule.create()],
        steps: [
          { id: "servers_1", kind: "servers", with: { targets: [], from: "", state: "running", usage: true } },
          {
            id: "foreach_1",
            kind: "foreach",
            with: { items: "{{steps.servers_1.servers}}", concurrency: 2 },
            steps: [
              {
                id: "if_1",
                kind: "if",
                with: { condition: rule("{{item.players}}", "eq", "0") },
                steps: [{ id: "restart_1", kind: "restart", with: { targets: [], from: "{{item}}", warnings: [], message: "" } }],
                else: [
                  {
                    id: "message_1",
                    kind: "message",
                    with: { targets: [], from: "{{item}}", kind: "chat", text: t("The nightly restart waits until nobody plays."), subtitle: "", player: "" },
                  },
                ],
              },
            ],
          },
        ],
      }),
  },
  {
    ...crashes,
    icon: ScrollIcon,
    draft: () =>
      draft(crashes, {
        triggers: [{ kind: "event", level: "warn", categories: ["servers"], contains: "crashed" }],
        steps: [
          {
            id: "notify_1",
            kind: "notify",
            with: { channel: "", level: "warn", message: t("{{server}} on {{node}} crashed: {{message}}", { server: "{{trigger.server.name}}", node: "{{trigger.node.name}}", message: "{{trigger.message}}" }) },
          },
          { id: "wait_1", kind: "wait", with: { for: "2", unit: "minutes" } },
          { id: "start_1", kind: "start", continue: true, with: { targets: [], from: "{{trigger.server}}" } },
        ],
      }),
  },
  {
    ...full,
    icon: GaugeIcon,
    draft: () =>
      draft(full, {
        triggers: [{ kind: "metric", measure: "players", value: 40, minutes: 2 }],
        steps: [
          { id: "start_1", kind: "start", with: { targets: [], from: "" } },
          {
            id: "message_1",
            kind: "message",
            with: { targets: [], from: "{{trigger.server}}", kind: "chat", text: t("More servers are opening for you."), subtitle: "", player: "" },
          },
        ],
      }),
  },
  {
    ...hooked,
    icon: WebhooksLogoIcon,
    draft: () =>
      draft(hooked, {
        triggers: [triggers.webhook.create()],
        steps: [
          {
            id: "if_1",
            kind: "if",
            with: { condition: rule("{{trigger.body.player}}", "matches", "^[A-Za-z0-9_]{3,16}$") },
            steps: [{ id: "command_1", kind: "command", with: { targets: [], from: "", commands: ["give {{trigger.body.player}} diamond 1"] } }],
            else: [{ id: "terminate_1", kind: "terminate", with: { failed: true, message: t("The call named no valid player."), result: "" } }],
          },
        ],
      }),
  },
]
