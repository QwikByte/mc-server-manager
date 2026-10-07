import { t } from "i18next"
import type { Schedule } from "./api"
import { locale } from "@/lib/i18n"

/** Weekdays in the order of the week, Monday first. */
export const weekdays = [1, 2, 3, 4, 5, 6, 0]

/** The short name of a weekday, 0 being Sunday, in the panel's language. */
export const dayName = (day: number) =>
  new Intl.DateTimeFormat(locale, { weekday: "short", timeZone: "UTC" }).format(Date.UTC(2023, 0, 1 + day))

/** A date as YYYY-MM-DD in the panel's language, e.g. "24 Dec 2026". */
export const formatDay = (date: string) =>
  new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone: "UTC" }).format(Date.parse(`${date}T00:00:00Z`))

/** Today as YYYY-MM-DD in the browser's time zone. */
export const today = () => new Date().toLocaleDateString("sv")

const same = (a: number[], b: number[]) => a.length === b.length && a.every((d) => b.includes(d))

/** Describes the days of a schedule, e.g. "Every day", "Days 1 and 15 of the month" or "On 24 Dec 2026". */
function describeDays(s: Schedule, and: Intl.ListFormat): string {
  if (s.dates?.length) {
    return s.dates.length > 3
      ? t("On {{count}} dates", { count: s.dates.length })
      : t("On {{dates}}", { dates: and.format(s.dates.map(formatDay)) })
  }
  if (s.monthDays?.length) {
    return t("Days {{days}} of the month", {
      count: s.monthDays.length,
      days: and.format(s.monthDays.map(String)),
      defaultValue_one: "Day {{days}} of the month",
    })
  }
  if (s.days.length === 0) return t("Every day")
  if (same(s.days, [1, 2, 3, 4, 5])) return t("Weekdays")
  if (same(s.days, [0, 6])) return t("Weekends")
  return and.format(weekdays.filter((d) => s.days.includes(d)).map(dayName))
}

/** Describes a schedule, e.g. "Every day at 04:00" or "Mon and Thu at 04:00 and 16:00". */
export function describeSchedule(s: Schedule): string {
  const and = new Intl.ListFormat(locale, { type: "conjunction" })
  const days = describeDays(s, and)
  return s.times.length > 4
    ? t("{{days}}, {{count}} times a day", { days, count: s.times.length })
    : t("{{days}} at {{times}}", { days, times: and.format(s.times) })
}

/** Whether the dates of a schedule all passed, after which its task turned itself off. */
export const datesPassed = (s: Schedule) => !!s.dates?.length && s.dates[s.dates.length - 1] < today()

/** Times of day every given number of hours, starting at midnight. */
export const everyHours = (hours: number) => Array.from({ length: 24 / hours }, (_, i) => `${String(i * hours).padStart(2, "0")}:00`)
