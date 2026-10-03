import { t } from "i18next"
import type { Schedule } from "./api"
import { locale } from "@/lib/i18n"

/** Weekdays in the order of the week, Monday first. */
export const weekdays = [1, 2, 3, 4, 5, 6, 0]

/** The short name of a weekday, 0 being Sunday, in the panel's language. */
export const dayName = (day: number) =>
  new Intl.DateTimeFormat(locale, { weekday: "short", timeZone: "UTC" }).format(Date.UTC(2023, 0, 1 + day))

const same = (a: number[], b: number[]) => a.length === b.length && a.every((d) => b.includes(d))

/** Describes a schedule, e.g. "Every day at 04:00" or "Mon and Thu at 04:00 and 16:00". */
export function describeSchedule(s: Schedule): string {
  const and = new Intl.ListFormat(locale, { type: "conjunction" })
  const days =
    s.days.length === 0
      ? t("Every day")
      : same(s.days, [1, 2, 3, 4, 5])
        ? t("Weekdays")
        : same(s.days, [0, 6])
          ? t("Weekends")
          : and.format(weekdays.filter((d) => s.days.includes(d)).map(dayName))
  return s.times.length > 4
    ? t("{{days}}, {{count}} times a day", { days, count: s.times.length })
    : t("{{days}} at {{times}}", { days, times: and.format(s.times) })
}

/** Times of day every given number of hours, starting at midnight. */
export const everyHours = (hours: number) => Array.from({ length: 24 / hours }, (_, i) => `${String(i * hours).padStart(2, "0")}:00`)
