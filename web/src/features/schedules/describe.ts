import type { Schedule } from "./api"

export const dayNames = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]
/** Weekdays in the order of the week, Monday first. */
export const weekdays = [1, 2, 3, 4, 5, 6, 0]

const and = new Intl.ListFormat("en", { type: "conjunction" })
const same = (a: number[], b: number[]) => a.length === b.length && a.every((d) => b.includes(d))

/** Describes a schedule, e.g. "Every day at 04:00" or "Mon and Thu at 04:00 and 16:00". */
export function describeSchedule(s: Schedule): string {
  const days =
    s.days.length === 0
      ? "Every day"
      : same(s.days, [1, 2, 3, 4, 5])
        ? "Weekdays"
        : same(s.days, [0, 6])
          ? "Weekends"
          : and.format(weekdays.filter((d) => s.days.includes(d)).map((d) => dayNames[d]))
  return s.times.length > 4 ? `${days}, ${s.times.length} times a day` : `${days} at ${and.format(s.times)}`
}

/** Times of day every given number of hours, starting at midnight. */
export const everyHours = (hours: number) =>
  Array.from({ length: 24 / hours }, (_, i) => `${String(i * hours).padStart(2, "0")}:00`)
