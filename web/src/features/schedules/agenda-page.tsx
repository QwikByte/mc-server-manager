import { CalendarDotsIcon, SpinnerGapIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { t } from "i18next"
import { ErrorCallout } from "@/components/callout"
import { EmptyState } from "@/components/empty-state"
import { TabIntro } from "@/components/hub-layout"
import { IconTile } from "@/components/icon-tile"
import { Section } from "@/components/section"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { jobs } from "@/features/backups/api"
import { type AutomationTask, useAutomationTasks } from "@/features/dashboard/tasks"
import { policies } from "@/features/policies/api"
import { dayOf, formatTime } from "@/lib/format"
import { locale } from "@/lib/i18n"
import { today } from "./describe"

/** A task with the times it runs on one day. */
interface Entry extends AutomationTask {
  times: string[]
}

/** The days of the next week that tasks run on, in the panel's time zone, with the tasks of each in order. */
function byDay(tasks: AutomationTask[], upcoming: { id: string; at: string }[]) {
  const days = new Map<string, Map<string, Entry>>()
  for (const { id, at } of upcoming.toSorted((a, b) => Date.parse(a.at) - Date.parse(b.at))) {
    const task = tasks.find((a) => a.task.id === id)
    if (!task) continue
    const day = dayOf(at)
    if (!days.has(day)) days.set(day, new Map())
    const entries = days.get(day)!
    if (!entries.has(id)) entries.set(id, { ...task, times: [] })
    entries.get(id)!.times.push(at)
  }
  return [...days].map(([day, entries]) => ({ day, entries: [...entries.values()] }))
}

function dayTitle(day: string) {
  const date = Date.parse(`${day}T12:00:00Z`)
  const name = new Intl.DateTimeFormat(locale, { weekday: "long", day: "numeric", month: "long", timeZone: "UTC" }).format(date)
  if (day === today()) return t("Today, {{date}}", { date: name })
  const tomorrow = dayOf(Date.now() + 86_400_000)
  return day === tomorrow ? t("Tomorrow, {{date}}", { date: name }) : name
}

const time = (at: string) => formatTime(at, { timeStyle: "short" })

/** The times of a task on a day, e.g. "04:00 · 16:00", or how often it runs if that is often. */
const describeTimes = (times: string[]) =>
  times.length > 3
    ? t("{{count}} times, {{first}} to {{last}}", { count: times.length, first: time(times[0]), last: time(times[times.length - 1]) })
    : times.map(time).join(" · ")

/** The Agenda tab of the automation: what runs now and what runs in the next 7 days. */
export function AgendaPage() {
  const access = useAccess()
  const [canJobs, canPolicies] = [access.can("backupjobs.view"), access.can("policies.view")]
  const tasks = useAutomationTasks()
  const upcoming = [useQuery({ ...jobs.upcomingQuery, enabled: canJobs }), useQuery({ ...policies.upcomingQuery, enabled: canPolicies })]
  // The tasks name the runs, so the agenda waits for them too.
  const queries = [
    ...upcoming,
    useQuery({ ...jobs.tasksQuery, enabled: canJobs }),
    useQuery({ ...policies.tasksQuery, enabled: canPolicies }),
  ]
  const error = queries.find((q) => q.error)?.error
  const running = tasks.filter((a) => a.task.running)
  const days = byDay(
    tasks,
    upcoming.flatMap((q) => q.data ?? []),
  )
  return (
    <>
      <TabIntro>{t("What backup jobs and schedules do in the next 7 days, in your time zone. Paused ones are left out.")}</TabIntro>
      {error ? (
        <ErrorCallout error={error} />
      ) : queries.some((q) => q.isLoading) ? (
        <Skeleton className="h-60 rounded-xl" />
      ) : running.length + days.length === 0 ? (
        <EmptyState icon={CalendarDotsIcon} tone="warning" title={t("Nothing runs in the next 7 days")} />
      ) : (
        <div className="grid gap-8">
          {running.length > 0 && <Day title={t("Running now")} entries={running.map((a) => ({ ...a, times: [] }))} />}
          {days.map(({ day, entries }) => (
            <Day key={day} title={dayTitle(day)} entries={entries} />
          ))}
        </div>
      )}
    </>
  )
}

function Day({ title, entries }: { title: string; entries: Entry[] }) {
  return (
    <Section title={title} className="mt-0">
      <ul className="surface divide-y rounded-xl">
        {entries.map(({ task, link, icon, tone, times }) => (
          <li key={task.id} className="flex items-center gap-3 px-4 py-2.5">
            <IconTile icon={icon} tone={tone} size="sm" />
            <div className="min-w-0 flex-1">
              <Link {...link} className="block truncate text-sm font-medium hover:underline">
                {task.name}
              </Link>
              <span className="block truncate text-xs text-muted-foreground">
                {link.to === "/backups/$jobId" ? t("Backup job") : t("Schedule")}
              </span>
            </div>
            <span className="max-w-36 text-right font-mono text-xs tabular-nums sm:max-w-none sm:text-sm">
              {times.length > 0 ? (
                describeTimes(times)
              ) : (
                <SpinnerGapIcon aria-hidden className="size-4 animate-spin text-muted-foreground" />
              )}
            </span>
          </li>
        ))}
      </ul>
    </Section>
  )
}
