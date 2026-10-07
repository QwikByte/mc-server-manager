import { ArrowClockwiseIcon, StopIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { type FormEvent, useState } from "react"
import { Segmented } from "@/components/segmented"
import { Button } from "@/components/ui/button"
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldContent, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import type { Warning } from "./api"

const minutes = ["1", "2", "5", "10"] as const

/**
 * Confirms that servers restart or stop, which can warn their players in the chat first: some minutes before, and
 * again 5 minutes and 1 minute before. Proxies get no warning.
 */
export function PowerDialog({
  action,
  title,
  description,
  warnable,
  canMessage,
  warnFirst = false,
  onConfirm,
  onOpenChange,
}: {
  action: "restart" | "stop"
  title: string
  description: string
  /** Whether game servers run among them, whose players can be warned. */
  warnable: boolean
  /** Whether the user may write the warning, which needs the permission to send console commands. */
  canMessage: boolean
  /** Whether the warning is on when the dialog opens. */
  warnFirst?: boolean
  onConfirm: (warning?: Warning) => void
  onOpenChange: (open: boolean) => void
}) {
  const [warn, setWarn] = useState(warnFirst && warnable)
  const [before, setBefore] = useState<(typeof minutes)[number]>("5")
  const [message, setMessage] = useState("")
  const stop = action === "stop"
  const Icon = stop ? StopIcon : ArrowClockwiseIcon
  const count = Number(before)
  const label = stop
    ? warn
      ? t("Stop in {{count}} min", { count })
      : t("Stop")
    : warn
      ? t("Restart in {{count}} min", { count })
      : t("Restart")

  function submit(event: FormEvent) {
    event.preventDefault()
    onConfirm(warn ? { minutes: count, message: message.trim() || undefined } : undefined)
    onOpenChange(false)
  }

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-6">
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{description}</DialogDescription>
          </DialogHeader>
          {warnable && (
            <div className="grid gap-4">
              <Field orientation="horizontal">
                <Switch id="power-warn" checked={warn} onCheckedChange={setWarn} />
                <FieldContent>
                  <FieldLabel htmlFor="power-warn">{t("Warn the players first")}</FieldLabel>
                  <FieldDescription>
                    {t("In the chat, and again 5 minutes and 1 minute before. Until then, it can be cancelled.")}
                  </FieldDescription>
                </FieldContent>
              </Field>
              {warn && (
                <>
                  <div className="flex flex-wrap items-center gap-3 text-sm">
                    <span>{t("Minutes before")}</span>
                    <Segmented
                      label={t("Minutes before")}
                      value={before}
                      onChange={setBefore}
                      options={minutes.map((m) => ({ value: m, label: m }))}
                    />
                  </div>
                  {canMessage && (
                    <Field>
                      <FieldLabel htmlFor="power-message">{t("Warning")}</FieldLabel>
                      <Input
                        id="power-message"
                        maxLength={200}
                        placeholder={stop ? t("The server stops in {minutes} min.") : t("The server restarts in {minutes} min.")}
                        value={message}
                        onChange={(e) => setMessage(e.target.value)}
                      />
                      <FieldDescription>
                        {t("Shown in the chat; {minutes} becomes the minutes left. Proxies get no warning.")}
                      </FieldDescription>
                    </Field>
                  )}
                </>
              )}
            </div>
          )}
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("Cancel")}</Button>
            </DialogClose>
            <Button type="submit" variant={stop ? "destructive" : "default"}>
              <Icon />
              {label}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
