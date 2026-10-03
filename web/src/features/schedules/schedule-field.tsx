import { PlusIcon, XIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Field, FieldDescription, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import type { Schedule } from "./api"
import { dayName, describeSchedule, everyHours, weekdays } from "./describe"

const timeZones = [...new Set(["UTC", ...Intl.supportedValuesOf("timeZone")])]
const repeats = [1, 3, 6, 12]

/** Edits when a task runs: the weekdays, the times of day and the time zone. */
export function ScheduleField({ value, onChange }: { value: Schedule; onChange: (schedule: Schedule) => void }) {
  const [time, setTime] = useState("12:00")
  const set = (change: Partial<Schedule>) => onChange({ ...value, ...change })
  const active = (day: number) => value.days.length === 0 || value.days.includes(day)

  function toggle(day: number) {
    const days = active(day) ? weekdays.filter((d) => d !== day && active(d)) : [...value.days, day]
    set({ days: days.length === 7 ? [] : days })
  }

  return (
    <>
      <FieldSet>
        <FieldLegend variant="label">{t("Days")}</FieldLegend>
        <div className="flex flex-wrap gap-1.5">
          {weekdays.map((day) => (
            <Button
              key={day}
              type="button"
              size="sm"
              variant={active(day) ? "default" : "outline"}
              aria-pressed={active(day)}
              // The last day stays, as a schedule without days would never run.
              disabled={active(day) && value.days.length === 1}
              className="w-12"
              onClick={() => toggle(day)}
            >
              {dayName(day)}
            </Button>
          ))}
        </div>
      </FieldSet>
      <FieldSet>
        <FieldLegend variant="label">{t("Times")}</FieldLegend>
        <ul className="flex flex-wrap gap-2">
          {value.times.map((at) => (
            <li key={at} className="flex items-center gap-1 rounded-lg bg-muted/70 py-1 pr-1 pl-2.5 font-mono text-sm font-medium">
              {at}
              <Button
                type="button"
                size="icon-xs"
                variant="ghost"
                aria-label={t("Remove {{time}}", { time: at })}
                disabled={value.times.length === 1}
                onClick={() => set({ times: value.times.filter((x) => x !== at) })}
              >
                <XIcon />
              </Button>
            </li>
          ))}
        </ul>
        <div className="flex flex-wrap items-center gap-2">
          <Input
            type="time"
            aria-label={t("Time of day")}
            className="w-32 font-mono"
            value={time}
            onChange={(e) => setTime(e.target.value)}
          />
          <Button
            type="button"
            variant="outline"
            disabled={!/^\d\d:\d\d$/.test(time) || value.times.includes(time)}
            onClick={() => set({ times: [...value.times, time].sort() })}
          >
            <PlusIcon />
            {t("Add time")}
          </Button>
          <span className="ml-1 text-xs text-muted-foreground">{t("or every")}</span>
          {repeats.map((hours) => (
            <Button key={hours} type="button" size="xs" variant="ghost" onClick={() => set({ times: everyHours(hours) })}>
              {hours === 1 ? t("hour") : t("{{hours}} h", { hours })}
            </Button>
          ))}
        </div>
      </FieldSet>
      <Field>
        <FieldLabel htmlFor="schedule-zone">{t("Time zone")}</FieldLabel>
        <Select value={value.timeZone} onValueChange={(timeZone) => set({ timeZone })}>
          <SelectTrigger id="schedule-zone" className="w-full sm:w-72">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {timeZones.map((zone) => (
              <SelectItem key={zone} value={zone}>
                {zone.replaceAll("_", " ")}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <FieldDescription>
          {t("{{schedule}}, {{zone}} time.", { schedule: describeSchedule(value), zone: value.timeZone.replaceAll("_", " ") })}
        </FieldDescription>
      </Field>
    </>
  )
}
