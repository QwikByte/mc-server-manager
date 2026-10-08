import { ArrowCounterClockwiseIcon, CheckIcon, PlusIcon, SlidersHorizontalIcon, SquaresFourIcon } from "@phosphor-icons/react"
import { useQuery } from "@tanstack/react-query"
import { t } from "i18next"
import { AnimatePresence, motion } from "motion/react"
import { useState } from "react"
import { toast } from "sonner"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccess } from "@/features/access/use-access"
import { meQuery } from "@/features/auth/api"
import { preferencesQuery, useSetDashboard, type Widget } from "@/features/preferences/api"
import { allServersQuery } from "@/features/servers/api"
import { formatTime, hourOf } from "@/lib/format"
import { useNow } from "@/lib/use-now"
import { WidgetGrid } from "./widget-grid"
import { arrange, widgetOf, widgets } from "./widgets"

/** The start page: widgets the user arranges, e.g. the players, servers and nodes at a glance and what needs attention. */
export function DashboardPage() {
  const access = useAccess()
  const { data: user } = useQuery(meQuery)
  const { data: servers } = useQuery(allServersQuery)
  const { data: preferences, isPending } = useQuery(preferencesQuery)
  const setDashboard = useSetDashboard()
  const [editing, setEditing] = useState(false)
  // The greeting follows the time of day.
  const now = new Date(useNow(true, 60_000))
  if (!servers || isPending) return <Skeleton className="h-96 rounded-xl" />

  const available = widgets.filter((w) => w.visible(access))
  const stored = preferences?.dashboard ?? []
  const layout = arrange(stored, available)
  // Widgets the user may not see now keep their place, for when they may again.
  const save = (next: Widget[]) => setDashboard.mutate([...next, ...stored.filter((w) => !available.some((d) => d.id === w.id))])
  const hidden = layout.filter((w) => w.hidden)

  function reset() {
    setDashboard.mutate([])
    toast.success(t("The overview has its default layout again."), {
      action: { label: t("Undo"), onClick: () => setDashboard.mutate(stored) },
    })
  }

  return (
    <>
      <PageHeader
        icon={SquaresFourIcon}
        title={greeting(user?.username ?? "", hourOf(now))}
        description={formatTime(now, { weekday: "long", day: "numeric", month: "long" })}
        actions={
          editing ? (
            <>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" disabled={hidden.length === 0}>
                    <PlusIcon />
                    {t("Add widget")}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-56">
                  <DropdownMenuLabel>{t("Hidden widgets")}</DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  {hidden.map((w) => {
                    const def = widgetOf(w.id)!
                    return (
                      <DropdownMenuItem key={w.id} onSelect={() => save(layout.map((l) => (l.id === w.id ? { id: l.id, columns: l.columns } : l)))}>
                        <def.icon weight="duotone" />
                        {t(def.title)}
                      </DropdownMenuItem>
                    )
                  })}
                </DropdownMenuContent>
              </DropdownMenu>
              <Button variant="ghost" onClick={reset} disabled={stored.length === 0}>
                <ArrowCounterClockwiseIcon />
                {t("Reset")}
              </Button>
              <Button onClick={() => setEditing(false)}>
                <CheckIcon />
                {t("Done")}
              </Button>
            </>
          ) : (
            <Button variant="outline" onClick={() => setEditing(true)}>
              <SlidersHorizontalIcon />
              {t("Customize")}
            </Button>
          )
        }
      />
      <AnimatePresence initial={false}>
        {editing && (
          <motion.p
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            className="overflow-hidden text-sm text-muted-foreground"
          >
            <span className="mb-8 block rounded-xl border border-dashed border-primary/40 bg-primary/5 px-4 py-3">
              {t("Drag widgets by their handle to rearrange them, change their width or hide them. The layout is saved to your account.")}
            </span>
          </motion.p>
        )}
      </AnimatePresence>
      <WidgetGrid layout={layout} editing={editing} onChange={save} />
    </>
  )
}

function greeting(name: string, hour: number) {
  if (hour < 5) return t("Still up, {{name}}?", { name })
  if (hour < 12) return t("Good morning, {{name}}", { name })
  if (hour < 18) return t("Good afternoon, {{name}}", { name })
  return t("Good evening, {{name}}", { name })
}
