import {
  ArrowDownIcon,
  ArrowUpIcon,
  CopySimpleIcon,
  DotsThreeVerticalIcon,
  EyeIcon,
  EyeSlashIcon,
  LightningIcon,
  PlusIcon,
  TrashIcon,
  WarningIcon,
} from "@phosphor-icons/react"
import { t } from "i18next"
import { createContext, type ReactNode, useContext, useState } from "react"
import { IconTile } from "@/components/icon-tile"
import { Pill } from "@/components/status"
import { Button } from "@/components/ui/button"
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { cn } from "@/lib/utils"
import type { Draft, Step, Trigger, TriggerKind } from "./api"
import { describeStep, describeTrigger, type Group, groups, kinds, missing, stepName, triggers } from "./catalog"
import { copyStep, insert, type ListKey, move, newStep, remove, replace, root } from "./tree"

/** What the cards of the flow change, and which one is open in the editor. */
interface Flow {
  draft: Draft
  change: (draft: Draft) => void
  /** The step or trigger open in the editor: a step's ID, or trigger:index. */
  selected?: string
  select: (id?: string) => void
  readOnly: boolean
}

const FlowContext = createContext<Flow | null>(null)
const useFlow = () => useContext(FlowContext) as Flow

/**
 * The flow of a workflow as cards from top to bottom, like a recipe: its triggers, then its steps, with a + between
 * them to add steps. Conditions split into Yes and No side by side, loops and branches hold their own steps.
 */
export function FlowEditor({ draft, change, selected, select, readOnly }: Flow) {
  const flow = { draft, change, selected, select, readOnly }
  return (
    <FlowContext.Provider value={flow}>
      <div className="flex flex-col items-center">
        <Triggers />
        <StepList steps={draft.steps} listKey={root} />
        <End />
      </div>
    </FlowContext.Provider>
  )
}

function Triggers() {
  const { draft, change, selected, select, readOnly } = useFlow()
  const add = (kind: TriggerKind) => {
    change({ ...draft, triggers: [...draft.triggers, triggers[kind].create()] })
    select(`trigger:${draft.triggers.length}`)
  }
  return (
    <div className="flex w-full max-w-md flex-col items-stretch gap-2">
      <p className="eyebrow text-center text-muted-foreground">{t("When")}</p>
      {draft.triggers.length === 0 && (
        <div className="rounded-xl border border-dashed px-4 py-3 text-center text-sm text-muted-foreground">
          {t("No trigger: it runs when you start it, or when another workflow runs it.")}
        </div>
      )}
      {draft.triggers.map((tr, i) => (
        // Triggers have no identity of their own and are only added at the end or removed.
        <TriggerCard key={i} trigger={tr} active={selected === `trigger:${i}`} onOpen={() => select(`trigger:${i}`)} />
      ))}
      {!readOnly && draft.triggers.length < 10 && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button type="button" variant="outline" size="sm" className="self-center">
              <PlusIcon />
              {t("Add a trigger")}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="center" className="w-72">
            {Object.entries(triggers).map(([kind, info]) => (
              <DropdownMenuItem key={kind} onSelect={() => add(kind as TriggerKind)} className="items-start">
                <info.icon className="mt-0.5 text-warning" />
                <span className="grid">
                  <span className="font-medium">{t(info.label)}</span>
                  <span className="text-xs text-muted-foreground">{t(info.description)}</span>
                </span>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  )
}

function TriggerCard({ trigger, active, onOpen }: { trigger: Trigger; active: boolean; onOpen: () => void }) {
  const info = triggers[trigger.kind]
  return (
    <button
      type="button"
      onClick={onOpen}
      className={cn(
        "surface lift flex items-center gap-3 rounded-xl p-3 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring",
        active && "ring-2 ring-primary",
      )}
    >
      <IconTile icon={info.icon} tone="warning" size="sm" />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-semibold">{t(info.label)}</span>
        <span className="block truncate text-xs text-muted-foreground">{describeTrigger(trigger)}</span>
      </span>
      <LightningIcon className="size-4 shrink-0 text-warning" aria-hidden />
    </button>
  )
}

/** A line between cards, with a + that adds a step there. */
function Gap({ listKey, index, last }: { listKey: ListKey; index: number; last?: boolean }) {
  const { readOnly } = useFlow()
  return (
    <div className="flex flex-col items-center" aria-hidden={readOnly}>
      <span className="h-3 w-px bg-border" />
      {readOnly ? <span className="h-2 w-px bg-border" /> : <AddStep listKey={listKey} index={index} />}
      {!last && <span className="h-3 w-px bg-border" />}
    </div>
  )
}

/** The steps of a list, from top to bottom. */
function StepList({ steps, listKey }: { steps: Step[]; listKey: ListKey }) {
  return (
    <ol className="flex w-full flex-col items-center">
      <li className="contents">
        <Gap listKey={listKey} index={0} last={steps.length === 0} />
      </li>
      {steps.map((s, i) => (
        <li key={s.id} className="flex w-full flex-col items-center">
          {kinds[s.kind]?.nests ? <Container step={s} /> : <StepCard step={s} />}
          <Gap listKey={listKey} index={i + 1} last={i === steps.length - 1} />
        </li>
      ))}
    </ol>
  )
}

/** The end of the flow. */
function End() {
  return <span className="mt-1 size-2.5 rounded-full bg-border" aria-hidden />
}

/** Chooses the kind of a step to add, by group, with a search. */
function AddStep({ listKey, index }: { listKey: ListKey; index: number }) {
  const { draft, change, select } = useFlow()
  const [open, setOpen] = useState(false)
  function add(kind: string) {
    const step = newStep(draft.steps, kind)
    change({ ...draft, steps: insert(draft.steps, listKey, index, step) })
    setOpen(false)
    select(step.id)
  }
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          type="button"
          size="icon-xs"
          variant="outline"
          className="rounded-full bg-card text-muted-foreground hover:border-primary hover:text-primary data-[state=open]:border-primary data-[state=open]:text-primary"
          aria-label={t("Add a step here")}
          title={t("Add a step here")}
        >
          <PlusIcon />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-96 p-0">
        <Command>
          <CommandInput placeholder={t("Search actions and controls…")} />
          <CommandList className="max-h-[26rem]">
            <CommandEmpty>{t("Nothing found.")}</CommandEmpty>
            {(Object.keys(groups) as Group[]).map((group) => (
              <CommandGroup key={group} heading={t(groups[group])}>
                {Object.entries(kinds)
                  .filter(([, info]) => info.group === group)
                  .map(([kind, info]) => (
                    <CommandItem key={kind} value={`${t(info.label)} ${t(info.description)} ${kind}`} onSelect={() => add(kind)} className="items-start">
                      <IconTile icon={info.icon} tone={info.tone} size="sm" className="size-7" />
                      <span className="grid min-w-0">
                        <span className="font-medium">{t(info.label)}</span>
                        <span className="text-xs text-muted-foreground">{t(info.description)}</span>
                      </span>
                    </CommandItem>
                  ))}
              </CommandGroup>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

/** The head of a step: its icon, name and what it does, which opens it in the editor, and its menu. */
function StepHead({ step, className }: { step: Step; className?: string }) {
  const { selected, select } = useFlow()
  const info = kinds[step.kind]
  const problem = missing(step)
  return (
    <div className={cn("flex items-center gap-2", className)}>
      <button
        type="button"
        onClick={() => select(step.id)}
        className="flex min-w-0 flex-1 items-center gap-3 rounded-lg text-left outline-none focus-visible:ring-2 focus-visible:ring-ring"
        aria-current={selected === step.id || undefined}
      >
        {info && <IconTile icon={info.icon} tone={step.disabled ? "neutral" : info.tone} size="sm" />}
        <span className="min-w-0 flex-1">
          <span className={cn("block truncate text-sm font-semibold", step.disabled && "text-muted-foreground line-through")}>{stepName(step)}</span>
          <span className="block truncate font-mono text-[0.6875rem] text-muted-foreground">{describeStep(step) || " "}</span>
        </span>
      </button>
      {problem && (
        <WarningIcon className="size-4 shrink-0 text-warning" aria-label={problem}>
          <title>{problem}</title>
        </WarningIcon>
      )}
      {step.continue && (
        <Pill tone="neutral" className="hidden sm:inline-flex">
          {t("Goes on")}
        </Pill>
      )}
      <StepMenu step={step} />
    </div>
  )
}

function StepMenu({ step }: { step: Step }) {
  const { draft, change, select, readOnly } = useFlow()
  if (readOnly) return null
  const steps = draft.steps
  const set = (next: Step[]) => change({ ...draft, steps: next })
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" size="icon-sm" variant="ghost" aria-label={t("Actions of {{step}}", { step: stepName(step) })} className="text-muted-foreground">
          <DotsThreeVerticalIcon />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onSelect={() => set(move(steps, step.id, -1))}>
          <ArrowUpIcon />
          {t("Move up")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => set(move(steps, step.id, 1))}>
          <ArrowDownIcon />
          {t("Move down")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => set(insertAfter(steps, step.id, copyStep(steps, step)))}>
          <CopySimpleIcon />
          {t("Duplicate")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => set(replace(steps, { ...step, disabled: !step.disabled }))}>
          {step.disabled ? <EyeIcon /> : <EyeSlashIcon />}
          {step.disabled ? t("Turn on") : t("Turn off")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          variant="destructive"
          onSelect={() => {
            set(remove(steps, step.id))
            select(undefined)
          }}
        >
          <TrashIcon />
          {t("Delete")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** Inserts a step right after another, in the same list. */
function insertAfter(steps: Step[], id: string, step: Step): Step[] {
  return steps.flatMap((s): Step[] => {
    const inner: Step = {
      ...s,
      ...(s.steps && { steps: insertAfter(s.steps, id, step) }),
      ...(s.else && { else: insertAfter(s.else, id, step) }),
      ...(s.cases && { cases: s.cases.map((c) => ({ ...c, steps: insertAfter(c.steps, id, step) })) }),
      ...(s.branches && { branches: s.branches.map((b) => insertAfter(b, id, step)) }),
    }
    return s.id === id ? [inner, step] : [inner]
  })
}

/** A step without steps within. */
function StepCard({ step }: { step: Step }) {
  const { selected } = useFlow()
  return (
    <div className={cn("surface w-full max-w-md rounded-xl p-3 transition-shadow", selected === step.id && "ring-2 ring-primary", step.disabled && "opacity-70")}>
      <StepHead step={step} />
    </div>
  )
}

/** A step that holds steps: in lanes side by side, e.g. Yes and No, or one lane below it, e.g. of a loop. */
function Container({ step }: { step: Step }) {
  const { selected } = useFlow()
  const nests = kinds[step.kind].nests ?? {}
  const lanes: { label: string; steps: Step[]; key: ListKey }[] = []
  if (nests.steps) lanes.push({ label: t(nests.steps), steps: step.steps ?? [], key: `${step.id}/steps` })
  step.cases?.forEach((c, i) => lanes.push({ label: t("Case {{value}}", { value: c.value || "…" }), steps: c.steps, key: `${step.id}/case/${i}` }))
  step.branches?.forEach((b, i) => lanes.push({ label: t("Branch {{number}}", { number: i + 1 }), steps: b, key: `${step.id}/branch/${i}` }))
  if (nests.else) lanes.push({ label: t(nests.else), steps: step.else ?? [], key: `${step.id}/else` })
  const side = lanes.length > 1
  return (
    <div
      className={cn(
        "w-full rounded-2xl bg-muted/40 p-2 ring-1 ring-border transition-shadow",
        selected === step.id && "ring-2 ring-primary",
        step.disabled && "opacity-70",
        side ? "max-w-none" : "max-w-xl",
      )}
    >
      <div className="surface mx-auto max-w-md rounded-xl p-3">
        <StepHead step={step} />
      </div>
      <div className={cn("mt-2 grid gap-2", side && (lanes.length === 2 ? "lg:grid-cols-2" : "lg:grid-cols-[repeat(auto-fit,minmax(18rem,1fr))]"))}>
        {lanes.map((lane) => (
          <Lane key={lane.key} label={lane.label} tone={laneTone(step.kind, lane.key)}>
            <StepList steps={lane.steps} listKey={lane.key} />
          </Lane>
        ))}
      </div>
    </div>
  )
}

/** Yes is green and No red, the catch of a try amber. */
function laneTone(kind: string, key: ListKey) {
  if (kind === "if") return key.endsWith("/steps") ? "text-success" : "text-destructive"
  if (kind === "try" && key.endsWith("/else")) return "text-warning"
  return "text-muted-foreground"
}

function Lane({ label, tone, children }: { label: string; tone: string; children: ReactNode }) {
  return (
    <section className="min-w-0 rounded-xl border border-dashed bg-background/60 px-2 pt-2 pb-3" aria-label={label}>
      <p className={cn("eyebrow truncate text-center", tone)}>{label}</p>
      {children}
    </section>
  )
}
