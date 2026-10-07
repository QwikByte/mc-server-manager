import { PlusIcon, XIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { type ReactNode, useState } from "react"
import { Segmented } from "@/components/segmented"
import { TimeZonePicker } from "@/components/time-zone-picker"
import { Button } from "@/components/ui/button"
import { Field, FieldDescription, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { formatTimeZone } from "@/lib/format"
import type { Schedule } from "./api"
import { dayName, describeSchedule, everyHours, formatDay, today, weekdays } from "./describe"

const repeats = [1, 3, 6, 12]
const monthDays = Array.from({ length: 31 }, (_, i) => i + 1)
const tomorrow = () => new Date(Date.now() + 86_400_000).toLocaleDateString("sv")

type Mode = "weekly" | "monthly" | "dates"

const modeOf = (s: Schedule): Mode => (s.dates?.length ? "dates" : s.monthDays?.length ? "monthly" : "weekly")

/** Edits when a task runs: on weekdays, days of the month or dates, the times of day and the time zone. */
export function ScheduleField({ value, onChange }: { value: Schedule; onChange: (schedule: Schedule) => void }) {
  const set = (change: Partial<Schedule>) => onChange({ ...value, ...change })
  const mode = modeOf(value)

  function choose(next: Mode) {
    // Each kind of days starts afresh, with a day that runs soon.
    const days = { days: [], monthDays: [], dates: [] }
    if (next === "monthly") set({ ...days, monthDays: [new Date().getDate()] })
    else if (next === "dates") set({ ...days, dates: [tomorrow()] })
    else set(days)
  }

  return (
    <>
      <FieldSet>
        <FieldLegend variant="label">{t("Days")}</FieldLegend>
        <Segmented
          label={t("Days")}
          value={mode}
          onChange={choose}
          className="w-fit"
          options={[
            { value: "weekly", label: t("Weekly") },
            { value: "monthly", label: t("Monthly") },
            { value: "dates", label: t("Single days") },
          ]}
        />
        {mode === "weekly" && <WeekdaysField value={value.days} onChange={(days) => set({ days })} />}
        {mode === "monthly" && <MonthDaysField value={value.monthDays ?? []} onChange={(monthDays) => set({ monthDays })} />}
        {mode === "dates" && <DatesField value={value.dates ?? []} onChange={(dates) => set({ dates })} />}
      </FieldSet>
      <TimesField value={value.times} onChange={(times) => set({ times })} />
      <Field>
        <FieldLabel htmlFor="schedule-zone">{t("Time zone")}</FieldLabel>
        {/* The picker gives UTC as an empty zone, schedules need its name. */}
        <TimeZonePicker
          id="schedule-zone"
          value={value.timeZone}
          onChange={(zone) => set({ timeZone: zone || "UTC" })}
          className="w-full sm:w-72"
        />
        <FieldDescription>
          {t("{{schedule}}, {{zone}} time.", { schedule: describeSchedule(value), zone: formatTimeZone(value.timeZone) })}
        </FieldDescription>
      </Field>
    </>
  )
}

/** A day that can be chosen or not; the last chosen one stays, as a schedule without days would never run. */
function DayButton({ on, last, onToggle, children }: { on: boolean; last: boolean; onToggle: () => void; children: ReactNode }) {
  return (
    <Button type="button" size="sm" variant={on ? "default" : "outline"} aria-pressed={on} disabled={on && last} onClick={onToggle}>
      {children}
    </Button>
  )
}

/** Weekdays, none meaning every day. */
function WeekdaysField({ value, onChange }: { value: number[]; onChange: (days: number[]) => void }) {
  const active = (day: number) => value.length === 0 || value.includes(day)
  function toggle(day: number) {
    const days = active(day) ? weekdays.filter((d) => d !== day && active(d)) : [...value, day]
    onChange(days.length === 7 ? [] : days)
  }
  return (
    <div className="flex flex-wrap gap-1.5 [&>button]:w-12">
      {weekdays.map((day) => (
        <DayButton key={day} on={active(day)} last={value.length === 1} onToggle={() => toggle(day)}>
          {dayName(day)}
        </DayButton>
      ))}
    </div>
  )
}

function MonthDaysField({ value, onChange }: { value: number[]; onChange: (days: number[]) => void }) {
  const toggle = (day: number) => onChange(value.includes(day) ? value.filter((d) => d !== day) : [...value, day].sort((a, b) => a - b))
  return (
    <>
      <div className="grid w-fit grid-cols-7 gap-1.5 [&>button]:w-9 [&>button]:px-0 [&>button]:font-mono">
        {monthDays.map((day) => (
          <DayButton key={day} on={value.includes(day)} last={value.length === 1} onToggle={() => toggle(day)}>
            {day}
          </DayButton>
        ))}
      </div>
      <FieldDescription>
        {t("A month without a chosen day, e.g. February without the 30th, runs on its last day instead.")}
      </FieldDescription>
    </>
  )
}

function DatesField({ value, onChange }: { value: string[]; onChange: (dates: string[]) => void }) {
  const [date, setDate] = useState(tomorrow)
  return (
    <>
      <Chips items={value} label={formatDay} onRemove={(d) => onChange(value.filter((x) => x !== d))} />
      <div className="flex flex-wrap items-center gap-2">
        <Input type="date" aria-label={t("Date")} className="w-44" min={today()} value={date} onChange={(e) => setDate(e.target.value)} />
        <Button
          type="button"
          variant="outline"
          disabled={!/^\d{4}-\d\d-\d\d$/.test(date) || value.includes(date) || value.length >= 100}
          onClick={() => onChange([...value, date].sort())}
        >
          <PlusIcon />
          {t("Add date")}
        </Button>
      </div>
      <FieldDescription>{t("After the last date, the task turns itself off.")}</FieldDescription>
    </>
  )
}

function TimesField({ value, onChange }: { value: string[]; onChange: (times: string[]) => void }) {
  const [time, setTime] = useState("12:00")
  return (
    <FieldSet>
      <FieldLegend variant="label">{t("Times")}</FieldLegend>
      <Chips items={value} label={(at) => at} onRemove={(at) => onChange(value.filter((x) => x !== at))} />
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
          disabled={!/^\d\d:\d\d$/.test(time) || value.includes(time)}
          onClick={() => onChange([...value, time].sort())}
        >
          <PlusIcon />
          {t("Add time")}
        </Button>
        <span className="ml-1 text-xs text-muted-foreground">{t("or every")}</span>
        {repeats.map((hours) => (
          <Button key={hours} type="button" size="xs" variant="ghost" onClick={() => onChange(everyHours(hours))}>
            {hours === 1 ? t("hour") : t("{{hours}} h", { hours })}
          </Button>
        ))}
      </div>
    </FieldSet>
  )
}

/** Times or dates that can be removed, except the last one. */
function Chips({ items, label, onRemove }: { items: string[]; label: (item: string) => string; onRemove: (item: string) => void }) {
  return (
    <ul className="flex flex-wrap gap-2">
      {items.map((item) => (
        <li key={item} className="flex items-center gap-1 rounded-lg bg-muted/70 py-1 pr-1 pl-2.5 font-mono text-sm font-medium">
          {label(item)}
          <Button
            type="button"
            size="icon-xs"
            variant="ghost"
            aria-label={t("Remove {{time}}", { time: label(item) })}
            disabled={items.length === 1}
            onClick={() => onRemove(item)}
          >
            <XIcon />
          </Button>
        </li>
      ))}
    </ul>
  )
}
